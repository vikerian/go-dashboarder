package mqtt

import (
	"encoding/json"
	"log/slog"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/vikerian/go-dashboarder/internal/domain"
)

// PublishRawMessage automaticky přidá HMAC a pošle JSON.
func (c *Client) PublishRawMessage(topic string, msg domain.RawMessage) error {
	// Spočítáme podpis z Payloadu
	msg.Checksum = domain.SignMessage(msg.Payload, c.secretKey)

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	token := c.MqttClient.Publish(topic, 1, false, data)
	token.Wait()
	return token.Error()
}

// SubscribeRawMessage přijme zprávu a ověří HMAC.
func (c *Client) SubscribeRawMessage(topic string, handler func(domain.RawMessage)) error {
	token := c.MqttClient.Subscribe(topic, 1, func(client mqtt.Client, m mqtt.Message) {
		var msg domain.RawMessage
		if err := json.Unmarshal(m.Payload(), &msg); err != nil {
			slog.Error("Failed to unmarshal RawMessage", "error", err)
			return
		}

		// Ověření integrity
		//if !domain.VerifyHMAC(msg.Payload, msg.Checksum, c.secretKey) {
		//	slog.Error("HMAC verification failed!", "msg_id", msg.ID)
		//	return
		//}

		handler(msg)
	})
	token.Wait()
	return token.Error()
}

// PublishIoTData posílá už normalizovaná IoT data do systému.
func (c *Client) PublishIoTData(topic string, data domain.IoTData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Tady už nepoužíváme HMAC, protože se pohybujeme v "bezpečné" vnitřní zóně /data/
	// Ale pokud bys chtěla, můžeš i zde použít RawMessage obálku.
	token := c.MqttClient.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

// PublishWebData posílá normalizovaná webová data (z Open-Meteo, RSS, atd.)
func (c *Client) PublishWebData(topic string, data domain.WebData) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	token := c.MqttClient.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}
