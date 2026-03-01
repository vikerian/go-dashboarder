package mqtt

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"dashboarder/internal/config" // Import našeho sdíleného configu

	paho "github.com/eclipse/paho.mqtt.golang"
)

// OptionsFactory vyrobí kompletní Paho options z naší config struktury.
// OptionsFactory je "mozkem" konfigurace. Vezme surový config a udělá z něj
// bezpečný objekt pro Paho knihovnu. Tím vyčistíme main.go od špinavé logiky.
func OptionsFactory(cfg config.MQTTConfig, clientID string) (*paho.ClientOptions, error) {
	// 1. Inicializace základních Paho voleb.
	opts := paho.NewClientOptions()
	opts.AddBroker(cfg.URL)
	opts.SetClientID(clientID)

	// 2. Nastavení retry mechanismu.
	// Bez AutoReconnect by komponenta při výpadku sítě "umřela" a už by se neprobudila.
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(10 * time.Second) // Reconnect se bude zrychlovat/zpomalovat
	opts.SetConnectTimeout(5 * time.Second)        // Nechceme, aby aplikace "visela" při startu

	// 3. Autentizace: Hesla posíláme jen pokud jsou definovaná.
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}

	// 4. Bezpečnost: Volání naší TLS továrny.
	// Tato funkce vrací *tls.Config nebo nil, pokud je TLS vypnuté.
	tlsCfg, err := TLSConfigFactory(
		cfg.UseTLS,
		cfg.InsecureSkipVerify,
		cfg.CAFile,
		cfg.CertFile,
		cfg.KeyFile,
	)
	if err != nil {
		// Pokud se nepodaří načíst certifikáty, aplikace se nesmí ani spustit!
		return nil, fmt.Errorf("tls configuration failed: %w", err)
	}

	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	// 5. Callbacky pro debugging (vnitřní mechanismus Paho).
	// Umožňuje nám vidět v logu, kdy přesně spojení spadlo.
	opts.SetOnConnectHandler(func(c paho.Client) {
		slog.Info("MQTT client connected successfully", "client_id", clientID)
	})

	opts.SetConnectionLostHandler(func(c paho.Client, err error) {
		slog.Warn("MQTT connection lost", "err", err, "client_id", clientID)
	})

	return opts, nil
}

// Client je náš interface.
type Client interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) error
	Subscribe(topic string, qos byte, callback func(topic string, payload []byte)) error
	Disconnect()
	// Tady je ta magická funkce, kterou voláš v main.go
	StartHeartbeat(ctx context.Context, topic string, interval time.Duration)
}

type mqttWrapper struct {
	client paho.Client // Hlavní klient pro data
	logger *slog.Logger
	topic  string
	// Uložíme si původní options, abychom mohli vytvořit "bráchu" pro heartbeat
	opts *paho.ClientOptions
}

func NewClient(opts *paho.ClientOptions, logger *slog.Logger, lwtTopic string) (Client, error) {
	// Nastavení LWT pro hlavní spojení
	lwtPayload, _ := json.Marshal(map[string]string{"status": "offline", "reason": "crash"})
	opts.SetWill(lwtTopic, string(lwtPayload), 1, true)

	c := paho.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return &mqttWrapper{client: c, logger: logger, topic: lwtTopic, opts: opts}, nil
}

// StartHeartbeat vytvoří vlastní, nezávislý kanál a spustí v něm gorutinu.
func (w *mqttWrapper) StartHeartbeat(ctx context.Context, topic string, interval time.Duration) {
	// 1. Vytvoříme si identické options jako má hlavní klient
	watchdogOpts := *w.opts                                 // Kopie původních nastavení (TLS, atd.)
	watchdogOpts.SetClientID(w.opts.ClientID + "-watchdog") // Musíme mít unikátní ID

	// 2. Připojíme nezávislého klienta
	watchdogClient := paho.NewClient(&watchdogOpts)
	if token := watchdogClient.Connect(); token.Wait() && token.Error() != nil {
		w.logger.Error("failed to start heartbeat channel", "err", token.Error())
		return
	}

	// 3. Spustíme gorutinu, která bude posílat statusy
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer watchdogClient.Disconnect(250)

		for {
			select {
			case <-ticker.C:
				payload, _ := json.Marshal(map[string]string{"status": "alive", "ts": time.Now().String()})
				token := watchdogClient.Publish(topic, 1, true, payload)
				token.Wait() // Tady čekáme, je to nezávislá gorutina, neblokuje hlavní app
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Publish automaticky serializuje payload do JSONu a pošle ho.
func (w *mqttWrapper) Publish(topic string, qos byte, retained bool, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		w.logger.Error("marshal error", "err", err)
		return err
	}

	// Publish je asynchronní operace.
	token := w.client.Publish(topic, qos, retained, data)

	// Spustíme asynchronní kontrolu v gorutině, abychom neblokovali hlavní vlákno.
	go func(t paho.Token) {
		if t.Wait() && t.Error() != nil {
			w.logger.Error("failed to publish", "topic", topic, "err", t.Error())
		}
	}(token)

	return nil
}

// Subscribe obaluje callback pro čistší API.
func (w *mqttWrapper) Subscribe(topic string, qos byte, callback func(topic string, payload []byte)) error {
	token := w.client.Subscribe(topic, qos, func(c paho.Client, m paho.Message) {
		callback(m.Topic(), m.Payload())
	})

	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// Disconnect pošle zprávu "offline" předtím, než se odpojí (korektní shutdown).
func (w *mqttWrapper) Disconnect() {
	w.Publish(w.topic, 1, true, map[string]string{"status": "offline", "reason": "shutdown"})
	w.client.Disconnect(250)
}

// TLSConfigFactory vytvoří konfiguraci pro TLS.
// - useTLS: zapne šifrování
// - insecure: dovolí self-signed certifikáty (POUZE PRO DEV!)
// - caFile, certFile, keyFile: cesty k souborům certifikátů
func TLSConfigFactory(useTLS bool, insecure bool, caFile, certFile, keyFile string) (*tls.Config, error) {
	if !useTLS {
		return nil, nil
	}

	// 1. Základní nastavení TLS (vyžadujeme moderní TLS 1.2+)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecure,
		MinVersion:         tls.VersionTLS12,
	}

	// 2. Načtení Root CA (pokud chceme validovat broker)
	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// 3. mTLS: Načtení identity klienta (certifikát + privátní klíč)
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
