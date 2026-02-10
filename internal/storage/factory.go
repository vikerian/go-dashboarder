package storage

import (
	"fmt"

	"github.com/vikerian/go-dashboarder/internal/config"
)

// NewStorage je naše Factory.
// Vrací interface 'Storage', takže volajícímu je jedno, co je uvnitř.
func NewStorage(cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "timescaledb", "postgres":
		return NewTimescaleStorage(cfg) // Voláme naši novou implementaci
	case "mock":
		return NewMockStorage(), nil
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
