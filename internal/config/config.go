// Package config zajišťuje jednotný přístup k nastavení všech mikroslužeb.
// Používá výhradně standardní Go knihovny, aby byl kompatibilní
// s jakýmkoliv prostředím (Docker/K8s/Bare metal).
package config

import (
	"os"
	"strconv"
	"strings"
)

// RootConfig je hlavní struktura, kterou načítají všechny komponenty.
type RootConfig struct {
	ComponentName string
	LogLevel      string

	// MQTT je univerzální nastavení pro jakékoliv MQTT spojení.
	MQTT MQTTConfig

	// APIConfig pro interní HTTP/gRPC servery.
	API APIConfig

	// WebConfig pro frontendy (dashboardy).
	Web WebConfig
}

type MQTTConfig struct {
	URL                string
	Username           string
	Password           string
	UseTLS             bool
	CAFile             string
	CertFile           string
	KeyFile            string
	InsecureSkipVerify bool // "DevOps Switch" pro self-signed/expired certy
}

type APIConfig struct {
	Port      int
	Timeout   int // v sekundách
	AuthToken string
}

type WebConfig struct {
	Port       int
	Timeout    int
	StaticPath string
	EnableAuth bool
	Username   string
	Password   string
}

// Load načte veškerou konfiguraci z environmentálních proměnných.
// V Kubernetes budou tyto proměnné definovány v Deployment manifestu.
func Load() RootConfig {
	return RootConfig{
		ComponentName: getEnv("COMPONENT_NAME", "go-service"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),

		MQTT: MQTTConfig{
			URL:                getEnv("MQTT_URL", "tcp://localhost:1883"),
			Username:           getEnv("MQTT_USER", ""),
			Password:           getEnv("MQTT_PASS", ""),
			UseTLS:             getEnvBool("MQTT_USE_TLS", false),
			CAFile:             getEnv("MQTT_CA_FILE", ""),
			CertFile:           getEnv("MQTT_CERT_FILE", ""),
			KeyFile:            getEnv("MQTT_KEY_FILE", ""),
			InsecureSkipVerify: getEnvBool("MQTT_INSECURE_SKIP_VERIFY", false),
		},

		API: APIConfig{
			Port:      getEnvInt("API_PORT", 3800),
			Timeout:   getEnvInt("API_TIMEOUT", 30),
			AuthToken: getEnv("API_AUTH_TOKEN", "K8zHbUMC/FgVBhvCYLIRziyZUMg3z880U0qXpZscDSgJ"),
		},

		Web: WebConfig{
			Port:       getEnvInt("WEB_PORT", 8080),
			Timeout:    getEnvInt("WEB_TIMEOUT", 30),
			StaticPath: getEnv("WEB_STATIC_PATH", "./web/dist"),
			EnableAuth: getEnvBool("WEB_ENABLE_AUTH", true),
			Username:   getEnv("WEB_USERNAME", "viewer"),
			Password:   getEnv("WEB_PASSWORD", "FlAqOBjVCAr@cMMUF2c4Z7S#j3iSKzIFcRoYvLCc78Vl"),
		},
	}
}

// --- Pomocné funkce pro typové načítání ---

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val := strings.ToLower(os.Getenv(key))
	if val == "true" || val == "1" || val == "yes" {
		return true
	}
	if val == "false" || val == "0" || val == "no" {
		return false
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return fallback
}
