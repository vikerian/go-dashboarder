#!/bin/bash

# 1. Cesta ke konfiguraci
export CONFIG_PATH="./configs/owm-input.yaml"

# 2. OWM SPECIFICKÉ PARAMETRY (Override YAML hodnot)
# Sem vlož svůj skutečný API klíč
export OWM_API_KEY=""
# Aktualne jsme presli na open-meteo. s api na openweathermap byl nejaky problem

# Souradnice (Bydliste)
export OWM_LAT=50.6537
export OWM_LON=14.0443

# Jak často se má modul ptát (v minutách)
export OWM_INTERVAL=5

# 3. MQTT PARAMETRY
export MQTT_CLIENT_ID="owm-ingester-praha"
export LOG_LEVEL="debug"

echo "--- Startuji OpenWeatherMap Ingester ---"
echo "Pozice: Lat $OWM_LAT, Lon $OWM_LON"
echo "Interval: $OWM_INTERVAL min"
echo "----------------------------------------"

# Spuštění modulu
go run cmd/owm-input/main.go
