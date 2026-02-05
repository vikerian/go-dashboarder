package models

type AplikacniStatus struct {
	Komponenta string `json:"component"`         // napr. web_api, timescaledb, mqtt_input, mqtt_intern...
	Status     string `json:"status"`            // "ok", "error", "warn"
	Message    string `json:"message,omitempty"` // doplnujici informace ke statusu
}
