#!/bin/bash
export CONFIG_PATH="./configs/mqtt-input.yaml"

# Pokud chceš změnit externí broker přes ENV:
export SOURCE_MQTT_URL="tcp://broker.hivemq.com:1883"
export SOURCE_TOPIC="vikerian/weather/prague"

go run cmd/mqtt-input/main.go
