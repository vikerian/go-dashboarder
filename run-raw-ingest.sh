#!/bin/bash

# 1. Nastavíme cestu ke konfiguraci
export CONFIG_PATH="./configs/raw-input.yaml"

# 2. EXPORT PARAMETRŮ DO ENVIRONMENTU (Override YAML hodnot)
# V YAML je log_level "debug", ale my ho pro tento běh změníme na "info"
export LOG_LEVEL="info"

# Změníme port, na kterém socket naslouchá (v YAML je 5000)
export RAW_LISTEN_PORT=6000

# Změníme ClientID pro MQTT, aby se nekrylo s jinou instancí
export MQTT_CLIENT_ID="raw-ingester-instance-alpha"

# Změníme tajný klíč na něco bezpečnějšího (tohle by v reálu byl Docker Secret)
export SECRET_KEY="moje-velmi-bezpecne-heslo-ktere-nikdo-neuhadne"

echo "--- Startuji modul s ENV parametry ---"
echo "Log Level: $LOG_LEVEL"
echo "Listen Port: $RAW_LISTEN_PORT"
echo "MQTT Client ID: $MQTT_CLIENT_ID"
echo "--------------------------------------"

# 3. SPUŠTĚNÍ APLIKACE
# Předpokládám, že jsi v kořenu projektu.
go run cmd/raw-input/main.go
