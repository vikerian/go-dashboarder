package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// MQTT definuje parametry pro připojení k brokeru.
// Používáme tagy pro YAML (soubor) i ENV (prostředí - např. Docker).
type MQTT struct {
	// BrokerURL: např. "tcp://localhost:1883" nebo "ssl://broker.hivemq.com:8883"
	BrokerURL string `yaml:"broker_url" env:"MQTT_URL" envDefault:"tcp://localhost:1883"`

	// ClientID: Unikátní identifikátor klienta. Důležité pro perzistentní session.
	ClientID string `yaml:"client_id" env:"MQTT_CLIENT_ID"`

	// Autentizace
	Username string `yaml:"username" env:"MQTT_USER"`
	Password string `yaml:"password" env:"MQTT_PASS"`

	// CleanSession: Pokud je false, broker si pamatuje nezpracované zprávy pro dané ClientID.
	CleanSession bool `yaml:"clean_session" env:"MQTT_CLEAN_SESSION" envDefault:"true"`

	// TLSConfig drží informace o šifrování a certifikátech (mTLS).
	TLS TLSConfig `yaml:"tls"`
}

// TLSConfig řeší bezpečnostní vrstvu.
type TLSConfig struct {
	Enabled            bool `yaml:"enabled" env:"MQTT_TLS_ENABLED" envDefault:"false"`
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" env:"MQTT_TLS_SKIP_VERIFY" envDefault:"false"`

	// Cesty k souborům s certifikáty
	CAFile   string `yaml:"ca_file" env:"MQTT_TLS_CA"`     // Root CA certifikát serveru
	CertFile string `yaml:"cert_file" env:"MQTT_TLS_CERT"` // Klientský certifikát (mTLS)
	KeyFile  string `yaml:"key_file" env:"MQTT_TLS_KEY"`   // Klientský soukromý klíč (mTLS)
}

// ToTLSConfig převede naši konfiguraci na standardní *tls.Config pro Go síťovou knihovnu.
// Toto je "Senior move" – logika transformace patří ke struktuře, která ji definuje.
func (c *TLSConfig) ToTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	t := &tls.Config{
		InsecureSkipVerify: c.InsecureSkipVerify,
	}

	// Načtení CA certifikátu pro ověření identity serveru
	if c.CAFile != "" {
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		t.RootCAs = caCertPool
	}

	// Načtení páru certifikát/klíč pro mTLS (ověření klienta serverem)
	if c.CertFile != "" && c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		t.Certificates = []tls.Certificate{cert}
	}

	return t, nil
}
