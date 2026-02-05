package main

import (
	"log/slog"
	"os"

	"github.com/vikerian/dashboarder-go/internal/config"

	"github.com/k0kubun/pp/v3"
)

// github.com/vikerian/go-dashboarder/internal/models

/* globalni promenne , aktualne logger protoze pouzijeme slog jako singleton */
var logger *slog.Logger

func main() {
	// inicializace logger singletonu
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// // tak se nejdriv privitame
	slog.Info("Dashboarder api application daemon booting up...")
	// pro jistotu ukoncovaci info
	defer slog.Info("Dashboarder api application daemon closing...")

	// instance konfigu
	cfg := config.NewConfig()
	// prozatim printneme pres pp
	pp.Printf("%+v\n", cfg)
}
