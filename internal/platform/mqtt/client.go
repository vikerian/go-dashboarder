package mqtt

import (
	"fmt"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/vikerian/go-dashboarder/internal/config"
)

// Client je náš robustní wrapper nad Paho MQTT.
type Client struct {
	MqttClient mqtt.Client
	cfg        config.MQTT
	secretKey  string
}

// NewClient vytvoří novou instanci, ale ještě se nepřipojuje.
func NewClient(cfg config.MQTT, secretKey string) (*Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.BrokerURL)
	opts.SetClientID(cfg.ClientID)
	opts.SetUsername(cfg.Username)
	opts.SetPassword(cfg.Password)
	opts.SetCleanSession(cfg.CleanSession)

	// Resilience nastavení
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Minute)
	opts.SetConnectRetry(true)

	// TLS
	tlsCfg, err := cfg.TLS.ToTLSConfig()
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	// Téma pro status modulu
	statusTopic := fmt.Sprintf("/status/%s", cfg.ClientID)

	// LWT - Last Will (Broker pošle tuto zprávu, pokud se spojení nečekaně přeruší)
	// Nastavujeme retain=true, aby noví klienti věděli, že jsme offline.
	opts.SetWill(statusTopic, `{"status": "offline"}`, 1, true)

	// OnConnectHandler - Tady řešíme "online" zprávu
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		slog.Info("MQTT connected", "broker", cfg.BrokerURL, "client_id", cfg.ClientID)

		// Jakmile se připojíme, pošleme "online" status s retain=true
		token := c.Publish(statusTopic, 1, true, `{"status": "online"}`)
		token.Wait()
		slog.Debug("Sent online status", "topic", statusTopic)
	})

	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		slog.Warn("MQTT connection lost", "error", err, "client_id", cfg.ClientID)
	}

	return &Client{
		MqttClient: mqtt.NewClient(opts),
		cfg:        cfg,
		secretKey:  secretKey,
	}, nil
}

func (c *Client) Connect() error {
	slog.Debug("Connecting to MQTT broker", "url", c.cfg.BrokerURL)
	token := c.MqttClient.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *Client) Disconnect() {
	// Při slušném odpojení bys technicky mohla poslat "offline" ručně,
	// ale LWT se postará o nečekané pády.
	c.MqttClient.Disconnect(250)
	slog.Info("MQTT disconnected", "client_id", c.cfg.ClientID)
}
