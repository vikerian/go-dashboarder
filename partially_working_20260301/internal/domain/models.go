package domain

import (
	"encoding/json"
	"time"
)

// RawMessage je základní komunikační jednotka mezi Ingesterem a Parserem
type RawMessage struct {
	// Unikátní identifikátor zprávy (UUID v4)
	ID string `json:"id"`

	// Název instance/modulu, který data přijal
	Source string `json:"source"`

	// Timestampy pro audit a kontrolu latence
	IngestedAt time.Time `json:"ingested_at"` // Čas vygenerování v ingesteru
	ReceivedAt time.Time `json:"received_at"` // Čas přijetí z vnějšího světa

	// Kontrolní součet pro integritu (např. SHA-256 z Payloadu)
	Checksum string `json:"checksum"`

	// Samotná data - stále v []byte pro maximální flexibilitu a výkon
	Payload []byte `json:"payload"`

	// u mqtt
	Topic string `json:"topic,omitempty"`
}

// IoTData představuje normalizovaný záznam pro IoT senzory.
// Tuto strukturu vyrábí IoT Parser a posílá ji do /data/iot.
type IoTData struct {
	DeviceID  string    `json:"device_id"`
	Metric    string    `json:"metric"` // např. "temperature", "humidity"
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`      // např. "°C", "%"
	Timestamp time.Time `json:"timestamp"` // Čas normalizace/měření
	Topic     string    `json:"topic"`
}

// WebData po revizi
type WebData struct {
	ID        string    `json:"id"` // Odkaz na původní RawMessage ID
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	FetchedAt time.Time `json:"fetched_at"`

	// Metadata jsou nyní typově bezpečnější pro transport
	// json.RawMessage zachová bajty, dokud je nebudeme chtít parsovat konkrétně
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// ComponentHealth reprezentuje jeden řádek v tabulce component_health
type ComponentHealth struct {
	Name     string    `json:"name"`
	Status   string    `json:"status"`
	LastSeen time.Time `json:"last_seen"`
	Message  string    `json:"message"`
}
