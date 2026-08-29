package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
	"github.com/vikerian/go-dashboarder/internal/logger"
	"github.com/vikerian/go-dashboarder/internal/platform/mqtt"

	paho "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	// 1. NAČTENÍ KONFIGURACE
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/iot-parser.yaml"
	}

	cfg, err := config.LoadConfig[config.BaseConfig](configPath)
	if err != nil {
		fmt.Printf("Fatal: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. INICIALIZACE LOGGERU
	logger.Setup(cfg.LogLevel, cfg.ComponentName, cfg.Environment == "development")

	slog.Info("Starting IoT Parser service", "component", cfg.ComponentName)

	// 3. PŘÍPRAVA MQTT KLIENTA
	client, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Failed to initialize MQTT client", "error", err)
		os.Exit(1)
	}

	if err := client.Connect(); err != nil {
		slog.Error("Failed to connect to MQTT broker", "error", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	// 3. SPUŠTĚNÍ HEARTBEATU
	// Tohle zajistí, že v Health tabulce uvidíš "OK" místo "UNKNOWN"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.StartHeartbeat(ctx, 30*time.Second)

	// 4. ODBĚR DAT
	token := client.Subscribe("input/#", 1, func(c paho.Client, m paho.Message) {
		// Zpracování musí běžet mimo callback: parseAndForward dál volá
		// Publish(...).Wait() na TOMTÉŽ klientovi, jehož čtecí smyčka tenhle
		// callback právě vyvolala - synchronní čekání na PUBACK by ji zaseklo
		// (paho.mqtt.golang callback nesmí blokovat).
		payload := m.Payload()
		topic := m.Topic()
		go func() {
			// 4a. Rozbalíme naši obálku RawMessage
			var msg domain.RawMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				slog.Error("Failed to unmarshal RawMessage", "topic", topic, "error", err)
				return
			}

			// 4b. Ruční verifikace HMAC (protože nepoužíváme wrapper)
			//if !domain.VerifyHMAC(msg.Payload, msg.Checksum, cfg.SecretKey) {
			//	slog.Error("HMAC verification failed! Message tampered or wrong key.", "id", msg.ID)
			//	return
			//}

			// 4c. Samotné parsování s ohledem na Topic
			parseAndForward(client, msg, topic)
		}()
	})

	if token.Wait() && token.Error() != nil {
		slog.Error("Failed to subscribe", "error", token.Error())
		os.Exit(1)
	}

	// 5. GRACEFUL SHUTDOWN
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("IoT Parser is listening and processing...")
	<-ctx.Done()
}

func parseAndForward(client *mqtt.Client, msg domain.RawMessage, topic string) {
	rawContent := string(msg.Payload)
	var metric string
	var valueStr string

	// LOGIKA ROZHODOVÁNÍ:
	// Pokud payload obsahuje ":", bereme metriku z něj (starý formát ze socketu).
	// Pokud ne, zkusíme metriku vytáhnout z konce topicu (nový formát z Bridge/TinyControl).
	if strings.Contains(rawContent, ":") {
		parts := strings.Split(rawContent, ":")
		metric = strings.TrimSpace(parts[0])
		valueStr = strings.TrimSpace(parts[1])
	} else {
		// Příklad: input/mqtt/tinycontrol/teplota_venku -> metric = "teplota_venku"
		topicParts := strings.Split(topic, "/")
		metric = topicParts[len(topicParts)-1]
		valueStr = strings.TrimSpace(rawContent)
	}

	// Převod hodnoty
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		slog.Error("Invalid numeric value", "metric", metric, "value", valueStr, "error", err)
		return
	}

	normalized := domain.IoTData{
		DeviceID:  msg.Source,
		Metric:    metric,
		Value:     value,
		Unit:      determineUnit(metric),
		Timestamp: time.Now(),
		Topic:     topic,
	}

	targetTopic := fmt.Sprintf("/data/iot/%s", metric)
	if err := client.PublishIoTData(targetTopic, normalized); err != nil {
		slog.Error("Failed to publish", "metric", metric, "error", err)
	} else {
		slog.Info("Data normalized", "source", msg.Source, "metric", metric, "value", value)
	}
}

func determineUnit(metric string) string {
	m := strings.ToLower(metric)
	// ds1/ds2 jsou DS18B20 čidla bridgovaná z /msh/internal_temp - jméno
	// metriky je jen ID senzoru, takže "temp"/"teplota" v něm nenajdeme.
	if strings.Contains(m, "temp") || strings.Contains(m, "teplota") || m == "ds1" || m == "ds2" {
		return "°C"
	}
	if strings.Contains(m, "hum") || strings.Contains(m, "vlhkost") {
		return "%"
	}
	if strings.Contains(m, "pres") || strings.Contains(m, "tlak") {
		return "hPa"
	}
	return "unknown"
}
