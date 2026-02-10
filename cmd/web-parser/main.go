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
)

// OpenMeteoResponse definuje jen to, co nás zajímá z JSONu Open-Meteo
type OpenMeteoResponse struct {
	CurrentWeather struct {
		Temperature float64 `json:"temperature"`
		Windspeed   float64 `json:"windspeed"`
		WeatherCode int     `json:"weathercode"`
	} `json:"current_weather"`
}

func main() {
	cfg, err := config.LoadConfig[config.BaseConfig]("configs/web-parser.yaml")
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	client, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Failed to init MQTT", "err", err)
		os.Exit(1)
	}
	if err := client.Connect(); err != nil {
		slog.Error("Failed to connect MQTT", "err", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// Odebíráme zprávy z webových ingesterů
	err = client.SubscribeRawMessage("/input/web/#", func(msg domain.RawMessage) {
		slog.Info("Web Parser received data", "from", msg.Source)

		// 1. Dekódování specifického JSONu z Open-Meteo
		var omResp OpenMeteoResponse
		if err := json.Unmarshal(msg.Payload, &omResp); err != nil {
			slog.Error("Failed to decode Open-Meteo JSON", "err", err)
			return
		}

		// 2. Transformace na naše WebData
		// Metadata uložíme jako RawMessage, abychom o nic nepřišli
		metadataJSON, _ := json.Marshal(omResp.CurrentWeather)

		webData := domain.WebData{
			ID:        msg.ID,
			URL:       "https://open-meteo.com",
			Title:     fmt.Sprintf("Weather for %s", msg.Source),
			FetchedAt: time.Now(),
			Metadata:  metadataJSON,
		}

		// 3. Publikování do systému
		targetTopic := fmt.Sprintf("/data/web/%s", msg.Source)
		if err := client.PublishWebData(targetTopic, webData); err != nil {
			slog.Error("Failed to publish web data", "err", err)
		} else {
			slog.Info("Web data normalized", "temp", omResp.CurrentWeather.Temperature)
		}
	})

	if err != nil {
		slog.Error("Subscription failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
