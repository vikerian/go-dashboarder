package main

import (
	"context"
	"encoding/json"
	"fmt"
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
	cfg, err := config.LoadConfig[config.WeatherIngesterConfig]("configs/weather.yaml")
	if err != nil {
		slog.Error("Chyba konfigurace", "err", err)
		os.Exit(1)
	}

	logger.Setup(cfg.LogLevel, cfg.ComponentName, true)

	// 2. Inicializace MQTT klienta (s tvým opraveným NewClientem)
	slog.Info("mqtt connection", "parameters", slog.StringValue(fmt.Sprintf("%+v", cfg)))
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

	// 3b. HTTP endpoint pro ruční vynucení obnovy (bez čekání na 15min tick) -
	// hodí se pro demo/ladění, kdy chceš vidět čerstvý bod hned.
	startRefreshServer(cfg.RefreshListenPort, mqttClient)

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

// startRefreshServer spustí malý HTTP server s POST /refresh, který spustí
// fetchAndPublish okamžitě, mimo pravidelný 15minutový ticker.
func startRefreshServer(port int, client *mqtt.Client) {
	mux := http.NewServeMux()
	mux.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}

		temp, err := fetchAndPublish(client)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "temperature": temp})
	})

	addr := fmt.Sprintf(":%d", port)
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("Refresh HTTP server failed", "err", err)
		}
	}()
	slog.Info("Manual refresh endpoint listening", "addr", addr)
}

func fetchAndPublish(client *mqtt.Client) (float64, error) {
	// Příklad URL pro Prahu (zjednodušeno pro Open-Meteo)
	url := "https://api.open-meteo.com/v1/forecast?latitude=50.0755&longitude=14.4378&current_weather=true"

	resp, err := http.Get(url)
	if err != nil {
		slog.Error("API fetch failed", "err", err)
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		CurrentWeather struct {
			Temperature float64 `json:"temperature"`
		} `json:"current_weather"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		slog.Error("JSON decode failed", "err", err)
		return 0, err
	}

	// Příprava dat pro náš systém
	// POZOR: Musí to odpovídat tomu, co Storager umí uložit
	data := domain.IoTData{
		Timestamp: time.Now(),
		DeviceID:  "weather-prague", // Toto bude v grafu jako jméno čáry
		Metric:    "temperature",
		Value:     result.CurrentWeather.Temperature,
		Unit:      "°C",
		Topic:     "/data/iot/weather",
	}

	payload, _ := json.Marshal(data)

	// Publikujeme do MQTT. Storager odebírá "/data/iot/#" a data uloží do DB
	// jako IoT záznam (weather-ingester posílá stejný domain.IoTData tvar).
	// Používáme QoS 1 pro spolehlivost.
	token := client.MqttClient.Publish("/data/iot/weather", 1, false, payload)
	token.Wait()

	slog.Info("Open-Meteo data successfully ingested", "temp", result.CurrentWeather.Temperature)
	return result.CurrentWeather.Temperature, nil
}
