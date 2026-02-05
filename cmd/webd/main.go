package main

import (
	"log/slog"
	"os"

	"github.com/vikerian/dashboarder-go/internal/config"

	// pretty printer please :)
	"github.com/k0kubun/pp/v3"
)

/* globalni promenne , aktualne logger protoze pouzijeme slog jako singleton */
var logger *slog.Logger

/* inicializacni funkce - nahodime logovani */
func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

/* hlavni funkce */
func main() {
	// tak se nejdriv privitame
	slog.Info("Dashboarder web application daemon booting up...")
	// pro jistotu ukoncovaci info
	defer slog.Info("Dashboarder web application daemon closing...")

	// instance konfigu
	cfg := config.NewConfig()
	// prozatim printneme pres pp
	pp.Printf("%+v\n", cfg)
}
