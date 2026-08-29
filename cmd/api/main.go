package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/logger"
	"github.com/vikerian/go-dashboarder/internal/platform/mqtt"
	"github.com/vikerian/go-dashboarder/internal/storage"
)

func main() {
	// 1. Načtení konfigurace
	cfg, _ := config.LoadConfig[config.BaseConfig]("configs/api.yaml")
	// Nastavíme logování na DEBUG natvrdo, ať vidíme všechno!
	logger.Setup("debug", cfg.ComponentName, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. DB
	db, err := storage.NewStorage(cfg.Storage)
	if err != nil {
		slog.Error("CRITICAL: DB connection failed", "err", err)
		os.Exit(1)
	}

	// 3. Health Manager - Zkrátíme interval na 30s pro rychlejší testování
	hManager := NewHealthManager(db, 30*time.Second)
	go hManager.StartChecker(ctx)

	// 4. MQTT Odběratel
	mqttClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("MQTT Client creation failed", "err", err)
	} else {
		if err := mqttClient.Connect(); err != nil {
			slog.Error("MQTT Connection failed - API is BLIND to status messages", "err", err)
		} else {
			// 3. SPUŠTĚNÍ HEARTBEATU
			// Tohle zajistí, že v Health tabulce uvidíš "OK" místo "UNKNOWN"
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			mqttClient.StartHeartbeat(ctx, 30*time.Second)

			// REGISTRACE ODBĚRU
			token := mqttClient.Subscribe("status/#", 1, func(c paho.Client, m paho.Message) {
				comp := strings.TrimPrefix(m.Topic(), "status/")
				payload := string(m.Payload())

				// TADY TO MUSÍŠ VIDĚT V LOGU!
				slog.Info("---> MQTT STATUS RECEIVED", "component", comp, "payload", payload)
				hManager.OnMessageReceived(comp, payload)
			})
			token.Wait()
			slog.Info("MQTT Subscription active for status/#")
		}
	}

	mux := http.NewServeMux()

	// Handler pro grafy a data
	mux.HandleFunc("/api/v1/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		kind := r.URL.Query().Get("kind")
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 {
			limit = 300
		}

		data, err := db.GetLatestEvents(r.Context(), kind, limit)
		if err != nil {
			slog.Error("Latest data fetch error", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	// Handler pro Health Dashboard
	mux.HandleFunc("/api/v1/health-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		healthData, err := db.GetComponentHealth(r.Context())
		if err != nil {
			slog.Error("Health data fetch error", "err", err)
			http.Error(w, "DB error", http.StatusInternalServerError)
			return
		}

		// Pro jistotu zalogujeme, co posíláme ven
		slog.Debug("Serving health data", "count", len(healthData))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthData)
	})

	slog.Info("API Server started", "port", 8080)
	http.ListenAndServe(":8080", mux)
}

// --- Health Manager zůstává stejný, jen přidáme logy ---

type HealthManager struct {
	db            storage.Storage
	heartbeats    map[string]time.Time
	mu            sync.Mutex
	checkInterval time.Duration
}

func NewHealthManager(db storage.Storage, interval time.Duration) *HealthManager {
	return &HealthManager{
		db:            db,
		heartbeats:    make(map[string]time.Time),
		checkInterval: interval,
	}
}

func (h *HealthManager) OnMessageReceived(component, payload string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if payload == "heartbeat" || payload == "alive" {
		h.heartbeats[component] = time.Now()
	}
}

func (h *HealthManager) StartChecker(ctx context.Context) {
	ticker := time.NewTicker(h.checkInterval)
	for {
		select {
		case <-ticker.C:
			h.evaluate(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *HealthManager) evaluate(ctx context.Context) {
	h.mu.Lock()
	current := make(map[string]time.Time)
	for k, v := range h.heartbeats {
		current[k] = v
	}
	h.mu.Unlock()

	slog.Info("--- Health Evaluation Cycle Starting ---", "known_components", len(current))

	for name, lastSeen := range current {
		status := "OK"
		if time.Since(lastSeen) > 2*time.Minute {
			status = "ERROR"
		}

		err := h.db.UpdateComponentHealth(ctx, name, status, "Last seen: "+lastSeen.Format("15:04:05"), lastSeen)
		if err != nil {
			slog.Error("DB Update failed", "comp", name, "err", err)
		} else {
			slog.Debug("Component status persisted", "comp", name, "status", status)
		}
	}
}
