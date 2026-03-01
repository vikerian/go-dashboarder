package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vikerian/go-dashboarder/internal/logger"
)

// Page definuje položku v bočním menu
type Page struct {
	Name string
	Link string
}

func main() {
	// Setup logování: "info" level, název komponenty, development mode = true
	logger.Setup("info", "frontend-portal", true)

	// Zjistíme aktuální pracovní adresář, abychom v logu viděli, kde jsme
	pwd, _ := os.Getwd()
	slog.Info("Frontend startuje", "working_dir", pwd)

	mux := http.NewServeMux()

	// 1. STATICKÉ SOUBORY (JS, CSS)
	// Servíruje soubory z /app/web/static pod URL cestou /static/
	fs := http.FileServer(http.Dir("/app/web/static"))
	mux.Handle("/static/", http.StripPrefix("/app/static/", fs))

	// 2. HLAVNÍ ROUTER PRO ŠABLONY
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Logování každého požadavku (jako middleware)
		start := time.Now()
		defer func() {
			slog.Info("Web Request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
		}()

		// Seznam stránek pro dynamické menu
		pages := getAvailablePages("/app/web/templates")

		// Očištění cesty: z "/facts" uděláme "facts"
		requestedPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestedPath == "" {
			requestedPath = "index" // Defaultní stránka
		}

		// Definice cest k souborům šablon
		layoutPath := filepath.Join("web", "templates", "layout.html")
		contentPath := filepath.Join("web", "templates", requestedPath+".html")

		// KONTROLA EXISTENCE SOUBORU
		if _, err := os.Stat(contentPath); os.IsNotExist(err) {
			slog.Warn("Stránka nenalezena", "path", contentPath)
			http.NotFound(w, r)
			return
		}

		// PARSOVÁNÍ ŠABLON (Layout + konkrétní Content)
		tmpl, err := template.ParseFiles(layoutPath, contentPath)
		if err != nil {
			slog.Error("Chyba parsování šablony", "err", err, "layout", layoutPath, "content", contentPath)
			http.Error(w, "Chyba na serveru při zpracování šablony", http.StatusInternalServerError)
			return
		}

		// DATA PRO ŠABLONU
		data := map[string]interface{}{
			"Pages":       pages,
			"CurrentPage": requestedPath,
		}

		// Vykreslení šablony (začínáme blokem "layout", který je v layout.html)
		if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
			slog.Error("Chyba renderování HTML", "err", err)
		}
	})

	slog.Info("Portál připraven", "port", 8180, "templates", "/app/web/templates")

	server := &http.Server{
		Addr:         ":8180",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Server selhal", "err", err)
		os.Exit(1)
	}
}

// getAvailablePages skenuje adresář a vrací seznam stránek pro menu
func getAvailablePages(dir string) []Page {
	var pages []Page
	files, err := os.ReadDir(dir)
	if err != nil {
		slog.Error("Nelze číst adresář se šablonami", "dir", dir, "err", err)
		return pages
	}

	for _, f := range files {
		name := f.Name()
		// Ignorujeme layout, index a vše co není .html
		if name == "layout.html" || name == "index.html" || !strings.HasSuffix(name, ".html") {
			continue
		}
		cleanName := strings.TrimSuffix(name, ".html")
		pages = append(pages, Page{
			Name: strings.Title(cleanName),
			Link: "/" + cleanName,
		})
	}
	return pages
}
