package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// RawIngesterConfig rozšiřuje BaseConfig o specifika socketu.
type RawIngesterConfig struct {
	BaseConfig `yaml:",inline"` // Vloží vše z BaseConfig na stejnou úroveň v YAML

	ListenPort int    `yaml:"listen_port" env:"RAW_LISTEN_PORT" envDefault:"5000"`
	Protocol   string `yaml:"protocol" env:"RAW_PROTOCOL" envDefault:"tcp"`
}

// MqttIngesterConfig pro odebírání dat z cizích MQTT serverů
type MqttIngesterConfig struct {
	BaseConfig `yaml:",inline"`

	// envPrefix zajistí, že tato struktura bude hledat např. SOURCE_MQTT_URL
	// namísto kolizního MQTT_URL
	SourceMQTT  MQTT   `yaml:"source_mqtt" envPrefix:"SOURCE_"`
	SourceTopic string `yaml:"source_topic" env:"SOURCE_TOPIC"`
}

type OpenWeatherConfig struct {
	BaseConfig `yaml:",inline"`

	APIKey   string  `yaml:"api_key" env:"OWM_API_KEY"`
	Lat      float64 `yaml:"lat" env:"OWM_LAT"`
	Lon      float64 `yaml:"lon" env:"OWM_LON"`
	Interval int     `yaml:"interval_minutes" env:"OWM_INTERVAL" envDefault:"15"`
}

// WeatherIngesterConfig rozšiřuje BaseConfig o port pro ruční vynucení
// obnovy dat (viz cmd/openmeteo-input) - normálně se stahuje jen jednou za
// 15 minut, tohle je pro demo/ladění, kdy chceš čerstvý bod hned.
type WeatherIngesterConfig struct {
	BaseConfig `yaml:",inline"`

	RefreshListenPort int `yaml:"refresh_listen_port" env:"WEATHER_REFRESH_PORT" envDefault:"6100"`
}

// WebScraperConfig pro tvůj scraper
type WebScraperConfig struct {
	BaseConfig `yaml:",inline"`

	TargetURL string `yaml:"target_url" env:"SCRAPE_URL"`
	Interval  int    `yaml:"interval_sec" env:"SCRAPE_INTERVAL" envDefault:"60"`
}

// SignMessage vytvoří HMAC podpis pro daný payload pomocí tajného klíče.
func SignMessage(payload []byte, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC zkontroluje, zda podpis odpovídá payloadu a klíči.
func VerifyHMAC(payload []byte, receivedSignature string, secretKey string) bool {
	expectedSignature := SignMessage(payload, secretKey)
	// Používáme ConstantTimeCompare, aby útočník nemohl použít "Timing Attack"
	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}
