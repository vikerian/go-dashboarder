package main

import (
	"context"
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
	"github.com/google/uuid"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/mqtt-input.yaml"
	}

	cfg, err := config.LoadConfig[config.MqttIngesterConfig](configPath)
	if err != nil {
		fmt.Printf("Fatal: %v\n", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	// 1. INTERNÍ KLIENT (náš systém)
	internalClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Internal MQTT init failed", "err", err)
		os.Exit(1)
	}

	if err := internalClient.Connect(); err != nil {
		slog.Error("Internal MQTT connect failed", "err", err)
		os.Exit(1)
	}
	defer internalClient.Disconnect()

	// 2. EXTERNÍ KLIENT (zdroj dat)
	sourceOpts := paho.NewClientOptions().
		AddBroker(cfg.SourceMQTT.BrokerURL).
		SetClientID(cfg.SourceMQTT.ClientID).
		SetAutoReconnect(true)

	// Nastavíme, co se má stát po (re)konektu k externímu zdroji
	sourceOpts.SetOnConnectHandler(func(client paho.Client) {
		slog.Info("Connected to external source", "broker", cfg.SourceMQTT.BrokerURL)

		// Odebíráme data z venku
		client.Subscribe(cfg.SourceTopic, 1, func(c paho.Client, m paho.Message) {
			slog.Debug("Data from external source", "topic", m.Topic())

			// Zabalíme do naší obálky
			raw := domain.RawMessage{
				ID:         uuid.NewString(),
				Source:     cfg.ComponentName,
				IngestedAt: time.Now(),
				ReceivedAt: time.Now(),
				Payload:    m.Payload(),
			}

			// Přeposíláme do našeho systému přes interního klienta
			targetTopic := fmt.Sprintf("/input/mqtt/%s", cfg.ComponentName)
			if err := internalClient.PublishRawMessage(targetTopic, raw); err != nil {
				slog.Error("Failed to bridge message", "error", err)
			}
		})
	})

	sourceClient := paho.NewClient(sourceOpts)
	if token := sourceClient.Connect(); token.Wait() && token.Error() != nil {
		slog.Warn("Initial source connection failed, retrying in background", "err", token.Error())
	}
	defer sourceClient.Disconnect(250)

	slog.Info("MQTT Bridge is running", "from", cfg.SourceTopic)

	// Shutdown logic
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
