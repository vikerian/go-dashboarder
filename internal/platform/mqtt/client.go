package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	// POZOR: Tato cesta musí odpovídat tvému go.mod!
	"github.com/vikerian/go-dashboarder/internal/config"
)

type subscription struct {
	topic   string
	qos     byte
	handler paho.MessageHandler
}

// Client obaluje paho MQTT klienta
type Client struct {
	MqttClient  paho.Client
	StatusTopic string
	ClientID    string
	cfg         config.MQTT
	secretKey   string

	subMu sync.Mutex
	subs  []subscription
}

// NewClient - Tady Go hledá config.MQTTConfig
func NewClient(cfg config.MQTT, secretKey string) (*Client, error) {
	c := &Client{
		StatusTopic: fmt.Sprintf("status/%s", cfg.ClientID),
		ClientID:    cfg.ClientID,
		cfg:         cfg,
		secretKey:   secretKey,
	}

	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(mc paho.Client) {
			// CleanSession=true znamená, že broker po každém (re)connectu
			// zahazuje staré odběry - musíme je tady znovu zaregistrovat,
			// jinak po výpadku/restartu brokeru klient tiše přestane
			// dostávat zprávy, i když vypadá jako připojený.
			c.resubscribeAll()
			mc.Publish(c.StatusTopic, 1, true, "alive")
		})

	// Nastavení LWT (Poslední vůle)
	opts.SetWill(c.StatusTopic, "dead", 1, true)

	c.MqttClient = paho.NewClient(opts)

	return c, nil
}

// Connect se pokusí o připojení. Odeslání úvodní "alive" zprávy a obnovu
// odběrů zajišťuje OnConnectHandler nastavený v NewClient - platí tedy i po
// každém automatickém reconnectu, ne jen při prvním připojení.
func (c *Client) Connect() error {
	slog.Debug("Connecting to MQTT broker", "url", c.cfg.BrokerURL)
	token := c.MqttClient.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

// Subscribe si pamatuje odběr, aby ho šlo po reconnectu obnovit, a hned ho
// i zařizuje. Volej tohle misto MqttClient.Subscribe přímo.
func (c *Client) Subscribe(topic string, qos byte, handler paho.MessageHandler) paho.Token {
	c.subMu.Lock()
	c.subs = append(c.subs, subscription{topic: topic, qos: qos, handler: handler})
	c.subMu.Unlock()

	return c.MqttClient.Subscribe(topic, qos, handler)
}

func (c *Client) resubscribeAll() {
	c.subMu.Lock()
	subs := make([]subscription, len(c.subs))
	copy(subs, c.subs)
	c.subMu.Unlock()

	for _, s := range subs {
		if token := c.MqttClient.Subscribe(s.topic, s.qos, s.handler); token.Wait() && token.Error() != nil {
			slog.Error("Failed to resubscribe after reconnect", "topic", s.topic, "err", token.Error())
		}
	}
}

// StartHeartbeat periodicky posílá pípnutí do MQTT
func (c *Client) StartHeartbeat(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.MqttClient.Publish(c.StatusTopic, 0, false, "heartbeat")
			case <-ctx.Done():
				c.MqttClient.Publish(c.StatusTopic, 1, true, "offline")
				return
			}
		}
	}()
}

func (c *Client) Disconnect() {
	c.MqttClient.Disconnect(250)
	slog.Info("MQTT disconnected", "client_id", c.cfg.ClientID)
}
