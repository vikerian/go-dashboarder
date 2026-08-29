#!/bin/sh
# mock_temp_gen.sh
# Simuluje DS18B20 čidla ds1/ds2 (rozsah 10.00-37.00 °C) přes mosquitto_pub.
# Pro pouziti mimo domaci RPi, kde normalne tahle data chodi z externiho
# MQTT zdroje do topicu /msh/internal_temp/ds1 a /msh/internal_temp/ds2.
#
# Konfigurace pres env promenne:
#   MQTT_HOST   - adresa brokeru (default: localhost)
#   MQTT_PORT   - port brokeru   (default: 1883)
#   TOPIC_BASE  - koren topicu   (default: /msh/internal_temp)
#   INTERVAL    - perioda v sekundach mezi publikacemi (default: 5)
#
# Pouziti:
#   ./mock_temp_gen.sh
#   MQTT_HOST=192.168.1.50 MQTT_PORT=1884 INTERVAL=2 ./mock_temp_gen.sh

MQTT_HOST="${MQTT_HOST:-localhost}"
MQTT_PORT="${MQTT_PORT:-1883}"
TOPIC_BASE="${TOPIC_BASE:-/msh/internal_temp}"
INTERVAL="${INTERVAL:-5}"

MIN_TEMP=10.00
MAX_TEMP=37.00

rand_temp() {
    # seed = cas + PID + iterace, aby se dve po sobe jdouci volani nesesli
    seed=$(( $(date +%s) + $$ + $1 ))
    awk -v min="$MIN_TEMP" -v max="$MAX_TEMP" -v seed="$seed" 'BEGIN {
        srand(seed);
        printf "%.2f", min + rand() * (max - min);
    }'
}

echo "mock_temp_gen.sh: publikuji na ${MQTT_HOST}:${MQTT_PORT}, topicy ${TOPIC_BASE}/ds1 a ${TOPIC_BASE}/ds2, kazdych ${INTERVAL}s (Ctrl+C pro konec)"

i=0
while true; do
    i=$((i + 1))
    val1=$(rand_temp "$i")
    val2=$(rand_temp "$((i + 500))")

    mosquitto_pub -h "$MQTT_HOST" -p "$MQTT_PORT" -t "${TOPIC_BASE}/ds1" -m "$val1"
    mosquitto_pub -h "$MQTT_HOST" -p "$MQTT_PORT" -t "${TOPIC_BASE}/ds2" -m "$val2"

    echo "$(date '+%H:%M:%S') ds1=${val1}  ds2=${val2}"
    sleep "$INTERVAL"
done

