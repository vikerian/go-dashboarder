package storage

import (
	"fmt"
	"log/slog"

	"github.com/vikerian/go-dashboarder/internal/config"
)

// NewStorage je naše Factory.
// Vrací interface 'Storage', takže volajícímu je jedno, co je uvnitř.
func NewStorage(cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "timescaledb":
		slog.Info("Initializing TimescaleDB storage", "host", cfg.Host)
		return NewTimescaleStorage(cfg)
	case "mock":
		slog.Warn("Using MOCK storage - NO DATA WILL BE READ FROM DB")
		return NewMockStorage(), nil
	default:
		// Místo vracení Mocku vrátíme chybu!
		return nil, fmt.Errorf("unknown storage type: %s (check your config/env)", cfg.Type)
	}
}
