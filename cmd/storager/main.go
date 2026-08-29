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

	// Náš wrapper si pojmenujeme 'internalMqtt'
	internalMqtt "github.com/vikerian/go-dashboarder/internal/platform/mqtt"
	"github.com/vikerian/go-dashboarder/internal/storage"

	// Knihovnu Paho si pojmenujeme 'paho'
	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	// 1. Config
	cfg, err := config.LoadConfig[config.BaseConfig]("configs/storager.yaml")
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}
	logger.Setup(cfg.LogLevel, cfg.ComponentName, true)

	// 2. Storage Factory
	db, err := storage.NewStorage(cfg.Storage)
	if err != nil {
		slog.Error("Failed to init storage", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// 3. Náš MQTT Klient (používáme alias internalMqtt)
	client, err := internalMqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Failed to init MQTT", "err", err)
		os.Exit(1)
	}

	if err := client.Connect(); err != nil {
		slog.Error("Failed to connect MQTT", "err", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// Tohle zajistí, že v Health tabulce uvidíš "OK" místo "UNKNOWN"
	hbCtx, hbCancel := context.WithCancel(context.Background())
	defer hbCancel()
	client.StartHeartbeat(hbCtx, 30*time.Second)

	// 4. Odebírání dat
	// Přístup k MqttClient (což je surový Paho klient) vyžaduje typy z paho
	client.Subscribe("/data/iot/#", 1, func(c paho.Client, m paho.Message) {
		var data domain.IoTData
		if err := json.Unmarshal(m.Payload(), &data); err != nil {
			slog.Error("Failed to unmarshal IoT data", "err", err)
			return
		}
		db.SaveIoT(context.Background(), data)
	})

	client.Subscribe("/data/web/#", 1, func(c paho.Client, m paho.Message) {
		var data domain.WebData
		if err := json.Unmarshal(m.Payload(), &data); err != nil {
			slog.Error("Failed to unmarshal Web data", "err", err)
			return
		}
		db.SaveWeb(context.Background(), data)
	})

	slog.Info("Storager is running and saving data...")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
