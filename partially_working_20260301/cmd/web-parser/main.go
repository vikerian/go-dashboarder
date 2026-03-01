package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
	"github.com/vikerian/go-dashboarder/internal/logger"
	"github.com/vikerian/go-dashboarder/internal/platform/mqtt"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Struktura odpovídající odpovědi z Open-Meteo
type OpenMeteoResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
		WeatherCode int     `json:"weathercode"`
	} `json:"current_weather"`
}

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/web-parser.yaml"
	}

	cfg, err := config.LoadConfig[config.BaseConfig](configPath)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	client, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("MQTT init failed", "err", err)
		os.Exit(1)
	}
	if err := client.Connect(); err != nil {
		slog.Error("MQTT connect failed", "err", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// 3. SPUŠTĚNÍ HEARTBEATU
	// Tohle zajistí, že v Health tabulce uvidíš "OK" místo "UNKNOWN"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.StartHeartbeat(ctx, 30*time.Second)

	// Odebíráme vše z input/web/#
	client.MqttClient.Subscribe("input/web/#", 1, func(c paho.Client, m paho.Message) {
		var raw domain.RawMessage
		if err := json.Unmarshal(m.Payload(), &raw); err != nil {
			return
		}

		// Ověření podpisu
		//if !domain.VerifyHMAC(raw.Payload, raw.Checksum, cfg.SecretKey) {
		//	slog.Error("HMAC verification failed for web data")
		//	return
		//}

		processWebData(client, raw)
	})

	slog.Info("Web Parser is active and waiting for JSON data...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

func processWebData(client *mqtt.Client, raw domain.RawMessage) {
	var weather OpenMeteoResponse
	if err := json.Unmarshal(raw.Payload, &weather); err != nil {
		slog.Error("Failed to parse Open-Meteo JSON", "err", err)
		return
	}

	// Vytvoříme normalizovaná data pro náš Storager
	// Použijeme WebData strukturu, kterou tvůj Storager ukládá jako kind='web'
	normalized := domain.WebData{
		URL:       raw.Source, // Tady bude název komponenty, např. "weather-prague"
		Title:     "Current Weather",
		FetchedAt: time.Now(),
		// Metadata uložíme jako JSON, aby se to v TimescaleDB uložilo do sloupce 'payload'
		Metadata: json.RawMessage(fmt.Sprintf(
			`{"value": %f, "windspeed": %f, "code": %d}`,
			weather.CurrentWeather.Temperature,
			weather.CurrentWeather.Windspeed,
			weather.CurrentWeather.WeatherCode,
		)),
	}

	// Pošleme do čisté zóny
	targetTopic := fmt.Sprintf("/data/web/%s", raw.Source)
	client.MqttClient.Publish(targetTopic, 1, false, mustMarshal(normalized))

	slog.Info("Web weather data normalized",
		"temp", weather.CurrentWeather.Temperature,
		"source", raw.Source)
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
