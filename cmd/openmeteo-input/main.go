package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
	"github.com/vikerian/go-dashboarder/internal/logger"
	"github.com/vikerian/go-dashboarder/internal/platform/mqtt"

	"github.com/google/uuid"
)

// Použijeme upravenou strukturu v internal/config/ingesters.go (odstraň APIKey)
func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/openmeteo-input.yaml"
	}

	cfg, err := config.LoadConfig[config.OpenWeatherConfig](configPath) // Použijeme stávající struct pro jednoduchost
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	mqttClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("MQTT init failed", "err", err)
		os.Exit(1)
	}
	if err := mqttClient.Connect(); err != nil {
		slog.Error("MQTT connect failed", "err", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Minute)
	defer ticker.Stop()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("Open-Meteo ingester started", "lat", cfg.Lat, "lon", cfg.Lon)
	slog.Info("MQTT Client info", "id", cfg.MQTT.ClientID, "status_topic", "/status/"+cfg.MQTT.ClientID)

	// První stažení
	fetchAndPublish(cfg, mqttClient)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fetchAndPublish(cfg, mqttClient)
		}
	}
}

func fetchAndPublish(cfg *config.OpenWeatherConfig, mqttClient *mqtt.Client) {
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true",
		cfg.Lat, cfg.Lon)

	// Seniorní tip: Vždy používej timeout u HTTP klienta
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		slog.Error("Network error calling Open-Meteo", "err", err)
		return
	}
	defer resp.Body.Close()

	// KONTROLA LIMITŮ A CHYB
	if resp.StatusCode == http.StatusTooManyRequests { // 429
		slog.Warn("Open-Meteo Rate Limit hit! Slow down, buddy.")
		return
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Open-Meteo returned error status", "status", resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read response body", "err", err)
		return
	}

	// Tady zkontroluj, jestli body není prázdné nebo neobsahuje JSON chybu
	if len(body) == 0 {
		slog.Warn("Received empty body from Open-Meteo")
		return
	}

	raw := domain.RawMessage{
		ID:         uuid.NewString(),
		Source:     cfg.ComponentName,
		IngestedAt: time.Now(),
		ReceivedAt: time.Now(),
		Payload:    body,
	}

	topic := fmt.Sprintf("/input/web/%s", cfg.ComponentName)
	if err := mqttClient.PublishRawMessage(topic, raw); err != nil {
		slog.Error("Failed to publish to MQTT", "err", err)
	} else {
		slog.Info("Open-Meteo data successfully ingested", "id", raw.ID)
	}
}
