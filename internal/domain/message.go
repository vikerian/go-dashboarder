package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID        uuid.UUID              `json:"id"`
	Source    string                 `json:"source"`             /// Zdroj dat (napr. ext_mqtt/channel/subchannel)
	Timestamp time.Time              `json:"timestamp"`          /// when received from mqtt
	MsgType   string                 `json:"msg_type"`           /// type of message (event/data/...)
	Metadata  map[string]string      `json:"metadata,omitempty"` /// Pro indexovatelné tagy (value_type, sensor_id, location, env)
	Payload   map[string]interface{} `json:"payload"`            /// Pro samotná data (value, status, raw_blob)
	Severity  string                 `json:"severity,omitempty"` /// critical/warn/info/data - pokud je type event, lze aplikovat severity, jinak ignore
}

// / NewMessage - constructor
func NewMessage(zdroj string, typ string, payload []byte) (msg *Message, err error) {
	// 1. Validace povinných polí – pokud chybí, nepustíme to dál
	if zdroj == "" {
		return nil, fmt.Errorf("zdroj (source) nesmí být prázdný")
	}
	if typ == "" {
		return nil, fmt.Errorf("typ (msg_type) nesmí být prázdný")
	}

	// 2. Unmarshal payloadu (vstup je []byte z MQTT, chceme map[string]interface{})
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(payload, &payloadMap); err != nil {
		return nil, fmt.Errorf("neplatný formát payloadu (není to JSON): %w", err)
	}

	// 3. Sestavení struktury
	msg = &Message{
		ID:        uuid.New(), // Unikátní ID pro trasování zpráv v DB
		Source:    zdroj,
		Timestamp: time.Now().UTC(), // Vždy v UTC, kvůli Timescale a časovým pásmům
		MsgType:   typ,
		Payload:   payloadMap,
		Metadata:  make(map[string]string), // Inicializujeme prázdnou mapu, ať ji můžeme plnit
	}

	return msg, nil
}
