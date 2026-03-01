package mqttclient

type MQTTConfig struct {
	BrokerURL string `yaml:"broker_url"`
	ClientID  string `yaml:"client_id"`
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	// TLS nastavení
	UseTLS   bool   `yaml:"use_tls"`
	CAFile   string `yaml:"ca_file"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	Topic    string `yaml:"topic"`
	Qos      int    `yaml:"qos"`
}
