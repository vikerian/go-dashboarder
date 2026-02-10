package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	// 1. NAČTENÍ KONFIGURACE
	// Cestu si bereme z ENV, abychom mohli v Dockeru snadno měnit soubory.
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/raw-input.yaml"
	}

	cfg, err := config.LoadConfig[config.RawIngesterConfig](configPath)
	if err != nil {
		// Tady ještě nemáme logger, tak holt postaru do stderr.
		fmt.Fprintf(os.Stderr, "FATAL: Nepodařilo se načíst konfiguraci: %v\n", err)
		os.Exit(1)
	}

	// 2. INICIALIZACE LOGGERU
	// Zapneme JSON logování, pokud nejsme v development módu.
	isDev := cfg.Environment == "development"
	logger.Setup(cfg.LogLevel, cfg.ComponentName, isDev)

	slog.Info("Startuji Raw Socket Ingester",
		"component", cfg.ComponentName,
		"port", cfg.ListenPort,
	)
	slog.Info("MQTT Konfigurace", "url", cfg.MQTT.BrokerURL, "client_id", cfg.MQTT.ClientID)

	// 3. PŘÍPRAVA MQTT KLIENTA
	// Používáme náš platformní wrapper, který v sobě má i HMAC a TLS.
	mqttClient, err := mqtt.NewClient(cfg.MQTT, cfg.SecretKey)
	if err != nil {
		slog.Error("Nepodařilo se vytvořit MQTT klienta", "error", err)
		os.Exit(1)
	}

	if err := mqttClient.Connect(); err != nil {
		slog.Error("Nepodařilo se připojit k MQTT brokeru", "error", err)
		os.Exit(1)
	}
	defer mqttClient.Disconnect() // Slušné vychování – zavřít spojení při konci.

	// 4. SPUŠTĚNÍ TCP SERVERU
	// Nasloucháme na portu definovaném v konfiguraci.
	addr := fmt.Sprintf(":%d", cfg.ListenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("Nepodařilo se otevřít TCP socket", "addr", addr, "error", err)
		os.Exit(1)
	}
	defer listener.Close()

	// 5. MECHANISMUS PRO GRACEFUL SHUTDOWN
	// Nechceme, aby se program prostě "ustřelil". Chceme dočíst rozdělanou práci.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Kanál pro chyby ze serveru, aby main věděl, že se něco rozbilo v goroutině.
	srvErr := make(chan error, 1)

	// Spustíme accept loop v goroutině
	go func() {
		slog.Info("TCP server naslouchá", "addr", addr)
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Pokud se listener zavře záměrně, vyhodíme chybu do kanálu.
				srvErr <- fmt.Errorf("accept error: %w", err)
				return
			}

			// Pro každé nové připojení nastartujeme separátní goroutinu (vlákno).
			// To je síla Go – tisíce spojení nejsou problém.
			go handleConnection(ctx, conn, mqttClient, cfg)
		}
	}()

	// 6. ČEKÁNÍ NA KONEC
	// Program tady visí a čeká, dokud nepřijde signál k ukončení nebo chyba serveru.
	select {
	case <-ctx.Done():
		slog.Info("Přijat signál k ukončení, vypínám server...")
	case err := <-srvErr:
		slog.Error("Kritická chyba serveru", "error", err)
	}
}

// handleConnection řeší komunikaci s jedním konkrétním připojeným senzorem.
func handleConnection(ctx context.Context, conn net.Conn, mqttClient *mqtt.Client, cfg *config.RawIngesterConfig) {
	defer conn.Close() // Vždy po sobě uklidit spojení.

	remoteAddr := conn.RemoteAddr().String()
	slog.Debug("Nové připojení", "remote_addr", remoteAddr)

	// Čteme data v loopu, dokud klient neukončí spojení nebo nás neukončí systém.
	buffer := make([]byte, 4096)
	for {
		// Nastavíme timeout pro čtení, aby nám tu spojení neviselo věčně.
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				slog.Debug("Chyba při čtení ze socketu", "remote_addr", remoteAddr, "error", err)
			}
			return // Klient se odpojil nebo vypršel timeout.
		}

		// Máme data! Teď z nich uděláme naši standardizovanou RawMessage.
		payload := make([]byte, n)
		copy(payload, buffer[:n])

		msg := domain.RawMessage{
			ID:         uuid.NewString(), // Generujeme unikátní ID pro tracing a dedup.
			Source:     cfg.ComponentName,
			IngestedAt: time.Now(),
			ReceivedAt: time.Now(),
			Payload:    payload,
		}

		// 7. ODESLÁNÍ DO SYSTÉMU
		// Používáme náš topic hierarchy: /input/raw/[jmeno_modulu]
		topic := fmt.Sprintf("/input/raw/%s", cfg.ComponentName)

		// PublishRawMessage v sobě automaticky spočítá HMAC podpis z Payloadu.
		if err := mqttClient.PublishRawMessage(topic, msg); err != nil {
			slog.Error("Nepodařilo se odeslat zprávu do MQTT", "msg_id", msg.ID, "error", err)
		} else {
			slog.Debug("Data úspěšně přijata a publikována",
				"msg_id", msg.ID,
				"size_bytes", n,
			)
		}
	}
}
