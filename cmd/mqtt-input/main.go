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
	internalMqtt "github.com/vikerian/go-dashboarder/internal/platform/mqtt"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

func main() {
	cfg, _ := config.LoadConfig[config.MqttIngesterConfig]("configs/mqtt-input.yaml")
	logger.Setup(cfg.LogLevel, cfg.ComponentName, true)

	// 1. NAŠ VNITŘNÍ KLIENT (na RPi #2)
	localClient, err := internalMqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Failed to init local MQTT", "err", err)
		os.Exit(1)
	}
	if err := localClient.Connect(); err != nil {
		slog.Error("Failed to connect local MQTT", "err", err)
		os.Exit(1)
	}

	// 2. EXTERNÍ KLIENT (Zdroj - RPi #1)
	sourceOpts := paho.NewClientOptions().
		AddBroker(cfg.SourceMQTT.BrokerURL).
		SetClientID(cfg.SourceMQTT.ClientID).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(c paho.Client) {
			slog.Info("Connected to SOURCE RPi MQTT", "url", cfg.SourceMQTT.BrokerURL)
			// Přihlásíme se k odběru dat z TinyControl
			c.Subscribe(cfg.SourceTopic, 1, func(client paho.Client, msg paho.Message) {
				bridgeMessage(localClient, msg, cfg.ComponentName)
			})
		})

	sourceClient := paho.NewClient(sourceOpts)
	if token := sourceClient.Connect(); token.Wait() && token.Error() != nil {
		slog.Error("Failed to connect source MQTT", "err", token.Error())
	}

	slog.Info("MQTT Bridge is active", "source", cfg.SourceTopic)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

func bridgeMessage(local *internalMqtt.Client, msg paho.Message, componentName string) {
	// Zabalíme data do naší obálky
	raw := domain.RawMessage{
		ID:         uuid.NewString(),
		Source:     fmt.Sprintf("bridge-%s", componentName),
		IngestedAt: time.Now(),
		ReceivedAt: time.Now(),
		Payload:    msg.Payload(),
	}

	// Pošleme to do našeho vnitřního systému (metoda PublishRawMessage přidá HMAC!)
	targetTopic := fmt.Sprintf("/input/mqtt/%s", msg.Topic())
	if err := local.PublishRawMessage(targetTopic, raw); err != nil {
		slog.Error("Bridge failed to forward message", "err", err)
	} else {
		slog.Debug("Bridged message", "topic", msg.Topic())
	}
}
