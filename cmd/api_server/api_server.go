package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/vikerian/go-dashboarder/internal/config"
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
	conf := config.NewConfig()
	// prozatim printneme pres pp
	slog.Debug(fmt.Sprintf("%+v", conf))
}
