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

func main() {
	// 1. Klasické kolečko: Config a Logger
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/owm-input.yaml"
	}

	cfg, err := config.LoadConfig[config.OpenWeatherConfig](configPath)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	// 2. Interní MQTT klient (náš systém)
	mqttClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Internal MQTT init failed", "err", err)
		os.Exit(1)
	}
	if err := mqttClient.Connect(); err != nil {
		slog.Error("Internal MQTT connect failed", "err", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	// 3. Ticker pro periodické stahování
	// OWM free limit je docela štědrý, ale raději nebuďme agresivní.
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Minute)
	defer ticker.Stop()

	// Shutdown mechanismus
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("OpenWeatherMap ingester started",
		"lat", cfg.Lat, "lon", cfg.Lon, "interval_min", cfg.Interval)

	// První stažení hned při startu, nečekáme na první "tik"
	fetchAndPublish(cfg, mqttClient)

	for {
		select {
		case <-ctx.Done():
			slog.Info("OWM ingester shutting down...")
			return
		case <-ticker.C:
			fetchAndPublish(cfg, mqttClient)
		}
	}
}

func fetchAndPublish(cfg *config.OpenWeatherConfig, mqttClient *mqtt.Client) {
	// Sestavení URL (používáme One Call API nebo aktuální počasí)
	url := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f&current_weather=true",
		cfg.Lat, cfg.Lon)

	// Stažení dat
	resp, err := http.Get(url)
	if err != nil {
		slog.Error("Failed to fetch from OWM", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("OWM returned non-200 status", "status", resp.Status)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read OWM body", "error", err)
		return
	}

	// Zabalení do naší standardní obálky
	raw := domain.RawMessage{
		ID:         uuid.NewString(),
		Source:     cfg.ComponentName,
		IngestedAt: time.Now(),
		ReceivedAt: time.Now(),
		Payload:    body, // Surový JSON od OpenWeatherMap
	}

	// Publikování do interního systému
	topic := fmt.Sprintf("/input/web/%s", cfg.ComponentName)
	if err := mqttClient.PublishRawMessage(topic, raw); err != nil {
		slog.Error("Failed to publish OWM data", "error", err)
	} else {
		slog.Info("OWM data ingested and published", "id", raw.ID)
	}
}
