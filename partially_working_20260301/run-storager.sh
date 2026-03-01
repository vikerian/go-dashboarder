#!/bin/bash

# 1. Cesta ke konfiguraci
export CONFIG_PATH="./configs/storager.yaml"

# 2. Případné overrides přes ENV
export LOG_LEVEL="info"

echo "--- Startuji Storager Service (Persistence Layer) ---"
echo "Storage Type: MOCK"
echo "----------------------------------------------------"

# 3. Spuštění
go run cmd/storager/main.go
