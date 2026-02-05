package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/vikerian/go-dashboarder/internal/config"
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
	conf := config.NewConfig()
	// prozatim printneme pres pp
	slog.Debug(fmt.Sprintf("%+v", conf))
}
