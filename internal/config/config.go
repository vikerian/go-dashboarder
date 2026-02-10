package config

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v10"
	"gopkg.in/yaml.v3"
)

type StorageConfig struct {
	Type     string `yaml:"type" env:"STORAGE_TYPE" envDefault:"mock"`
	Host     string `yaml:"host" env:"DB_HOST"`
	Port     int    `yaml:"port" env:"DB_PORT"`
	User     string `yaml:"user" env:"DB_USER"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	DBName   string `yaml:"dbname" env:"DB_NAME"`
}

// BaseConfig jsou parametry, které má KAŽDÝ modul v tvém systému.
// Používáme inline vnoření, aby YAML vypadal čistě.
type BaseConfig struct {
	ComponentName string `yaml:"component_name" env:"COMPONENT_NAME"`
	Environment   string `yaml:"environment" env:"ENV" envDefault:"development"`
	LogLevel      string `yaml:"log_level" env:"LOG_LEVEL" envDefault:"info"`

	// Každý modul u tebe mluví přes MQTT
	MQTT      MQTT   `yaml:"mqtt"`
	SecretKey string `yaml:"secret_key" env:"SECRET_KEY"`

	Storage StorageConfig `yaml:"storage"`
}

// LoadConfig je "Generic" funkce (to [T any]).
// Umožňuje nám načíst jakoukoliv strukturu, která "rozšiřuje" BaseConfig.
func LoadConfig[T any](path string) (*T, error) {
	// Vytvoříme novou instanci typu T
	cfg := new(T)

	// 1. Krok: Zkusíme načíst YAML soubor (pokud existuje)
	// YAML je skvělý pro výchozí hodnoty a lokální vývoj.
	if _, err := os.Stat(path); err == nil {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open config: %w", err)
		}
		defer file.Close()

		if err := yaml.NewDecoder(file).Decode(cfg); err != nil {
			return nil, fmt.Errorf("failed to decode yaml: %w", err)
		}
	}

	// 2. Krok: Přepíšeme hodnoty z Environment Variables.
	// V Dockeru/Kubernetes je toto klíčové. ENV má vždy přednost před souborem.
	// Knihovna 'env' projde strukturu a hledá tagy `env:"..."`.
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return cfg, nil
}
