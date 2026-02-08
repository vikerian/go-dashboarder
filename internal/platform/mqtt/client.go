package mqtt

import (
	"fmt"
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

	// LWT - Last Will
	statusTopic := fmt.Sprintf("/status/%s", cfg.ClientID)
	opts.SetWill(statusTopic, `{"status": "offline"}`, 1, false)

	return &Client{
		MqttClient: mqtt.NewClient(opts),
		cfg:        cfg,
		secretKey:  secretKey,
	}, nil
}

// SetOnConnect nám dovolí definovat, co se stane po každém úspěšném spojení.
func (c *Client) SetOnConnect(handler func(mqtt.Client)) {
	// Musíme to nastavit v Options předtím, než zavoláme Connect(),
	// nebo (pokud už běžíme) musíme použít interní mechanismy Paho.
	// Pro jednoduchost to teď nastavujeme přímo v main přes Paho Options
	// nebo zde v rámci našeho wrapperu.
}

func (c *Client) Connect() error {
	token := c.MqttClient.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}

func (c *Client) Disconnect() {
	c.MqttClient.Disconnect(250)
}
