package storage

import (
	"context"
	"time"

	"github.com/vikerian/go-dashboarder/internal/domain"
)

// Storage definuje, co všechno musíme umět s daty udělat.
type Storage interface {
	SaveIoT(ctx context.Context, data domain.IoTData) error
	SaveWeb(ctx context.Context, data domain.WebData) error
	GetLatestEvents(ctx context.Context, kind string, limit int) ([]domain.IoTData, error)
	GetEventsByMetric(ctx context.Context, metric string, hours int) ([]domain.IoTData, error)
	// UpdateComponentHealth zapíše stav do dedikované tabulky
	UpdateComponentHealth(ctx context.Context, name, status, msg string, lastSeen time.Time) error
	// GetComponentHealth vytáhne aktuální stavy všech komponent pro frontend
	GetComponentHealth(ctx context.Context) ([]domain.ComponentHealth, error)
	Close() error
}
