package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
	"github.com/vikerian/go-dashboarder/internal/logger"
	"github.com/vikerian/go-dashboarder/internal/platform/mqtt"
)

func main() {
	// 1. Načtení konfigurace (používá tvůj oblíbený env/v10)
	cfg, err := config.LoadConfig[config.BaseConfig]("configs/weather.yaml")
	if err != nil {
		slog.Error("Chyba konfigurace", "err", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, true)

	// 2. Inicializace MQTT klienta (s tvým opraveným NewClientem)
	mqttClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("MQTT init failed", "err", err)
		os.Exit(1)
	}

	if err := mqttClient.Connect(); err != nil {
		slog.Error("MQTT connection failed", "err", err)
		os.Exit(1)
	}

	// 3. SPUŠTĚNÍ HEARTBEATU
	// Tohle zajistí, že v Health tabulce uvidíš "OK" místo "UNKNOWN"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mqttClient.StartHeartbeat(ctx, 30*time.Second)

	slog.Info("Open-Meteo ingester started",
		"component", cfg.ComponentName,
		"id", cfg.MQTT.ClientID)

	// 4. Hlavní smyčka pro čerpání počasí
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// Provedeme první načtení hned při startu
	fetchAndPublish(mqttClient)

	for {
		select {
		case <-ticker.C:
			fetchAndPublish(mqttClient)
		case <-ctx.Done():
			return
		}
	}
}

func fetchAndPublish(client *mqtt.Client) {
	// Příklad URL pro Prahu (zjednodušeno pro Open-Meteo)
	url := "https://api.open-meteo.com/v1/forecast?latitude=50.0755&longitude=14.4378&current_weather=true"

	resp, err := http.Get(url)
	if err != nil {
		slog.Error("API fetch failed", "err", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		CurrentWeather struct {
			Temperature float64 `json:"temperature"`
		} `json:"current_weather"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("JSON decode failed", "err", err)
		return
	}

	// Příprava dat pro náš systém
	// POZOR: Musí to odpovídat tomu, co Storager umí uložit
	data := domain.IoTData{
		Timestamp: time.Now(),
		DeviceID:  "weather-prague", // Toto bude v grafu jako jméno čáry
		Metric:    "temperature",
		Value:     result.CurrentWeather.Temperature,
		Unit:      "°C",
		Topic:     "/data/weather",
	}

	payload, _ := json.Marshal(data)

	// Publikujeme do MQTT. Storager to zachytí a uloží do DB.
	// Používáme QoS 1 pro spolehlivost.
	token := client.MqttClient.Publish("/data/weather", 1, false, payload)
	token.Wait()

	slog.Info("Open-Meteo data successfully ingested", "temp", result.CurrentWeather.Temperature)
}
