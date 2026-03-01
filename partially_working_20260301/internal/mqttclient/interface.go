package mqttclient

type Client interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) error
	Subscribe(topic string, qos byte, callback func(topic string, payload []byte)) error
	Connect() error
	Disconnect()
}
