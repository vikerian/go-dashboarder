package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	// POZOR: Tato cesta musí odpovídat tvému go.mod!
	"github.com/vikerian/go-dashboarder/internal/config"
)

// Client obaluje paho MQTT klienta
type Client struct {
	MqttClient  paho.Client
	StatusTopic string
	ClientID    string
	cfg         config.MQTT
	secretKey   string
}

// NewClient - Tady Go hledá config.MQTTConfig
func NewClient(cfg config.MQTT, secretKey string) (*Client, error) {
	opts := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(true)

	// Nastavení LWT (Poslední vůle)
	statusTopic := fmt.Sprintf("status/%s", cfg.ClientID)
	opts.SetWill(statusTopic, "dead", 1, true)

	c := paho.NewClient(opts)

	return &Client{
		MqttClient:  c,
		StatusTopic: statusTopic,
		ClientID:    cfg.ClientID,
	}, nil
}

// Connect se pokusí o připojení a pošle úvodní "alive" status
func (c *Client) Connect() error {
	slog.Debug("Connecting to MQTT broker", "url", c.cfg.BrokerURL)
	token := c.MqttClient.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	// Pošleme retained zprávu, že jsme online
	c.MqttClient.Publish(c.StatusTopic, 1, true, "alive")
	return nil
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
