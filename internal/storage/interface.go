package storage

import (
	"context"

	"github.com/vikerian/go-dashboarder/internal/domain"
)

// Storage definuje, co všechno musíme umět s daty udělat.
type Storage interface {
	SaveIoT(ctx context.Context, data domain.IoTData) error
	SaveWeb(ctx context.Context, data domain.WebData) error
	Close() error
}
