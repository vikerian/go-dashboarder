package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Setup zinicializuje globální logger pro celou aplikaci.
// levelStr: "debug", "info", "warn", "error"
// component: název modulu (např. "mqtt-ingester")
// isDev: pokud je true, logujeme do čitelného textu, jinak do JSONu (audit log)
func Setup(levelStr string, component string, isDev bool) {
	var level slog.Level

	// 1. Převod stringu na slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 2. Nastavení handleru (Text vs JSON)
	opts := &slog.HandlerOptions{
		Level: level,
		// Tady můžeme přidat i zobrazení zdrojového souboru v logu (v debugu užitečné)
		AddSource: isDev,
	}

	var handler slog.Handler
	if isDev {
		// Formát pro lidi - přehledný v terminálu
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// Formát pro produkci/audit - JSON je standard pro log-collectory (ELK, Loki)
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	// 3. Obohacení loggeru o jméno komponenty
	// Každý log z tohoto loggeru teď bude mít v sobě "component": "..."
	logger := slog.New(handler).With(
		slog.String("component", component),
	)

	// 4. Nastavení jako výchozí globální logger
	slog.SetDefault(logger)
}

func Audit(msg string, args ...any) {
	// V Go můžeme přidat vlastní atributy, které označí log jako Audit
	args = append(args, slog.Bool("audit", true))
	slog.Info(msg, args...)
}
