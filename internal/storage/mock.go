package storage

import (
	"context"
	"time"

	"github.com/vikerian/go-dashboarder/internal/domain"
)

type MockStorage struct{}

func NewMockStorage() *MockStorage { return &MockStorage{} }

func (m *MockStorage) SaveIoT(ctx context.Context, data domain.IoTData) error { return nil }
func (m *MockStorage) SaveWeb(ctx context.Context, data domain.WebData) error { return nil }

// MUSÍ mít novou signaturu s kind string
func (m *MockStorage) GetLatestEvents(ctx context.Context, kind string, limit int) ([]domain.IoTData, error) {
	return []domain.IoTData{}, nil
}

func (m *MockStorage) GetEventsByMetric(ctx context.Context, metric string, hours int) ([]domain.IoTData, error) {
	return []domain.IoTData{}, nil
}

func (m *MockStorage) Close() error { return nil }

// Doplň tyto prázdné funkce do svého Mocku
func (m *MockStorage) UpdateComponentHealth(ctx context.Context, name, status, msg string, lastSeen time.Time) error {
	return nil
}

func (m *MockStorage) GetComponentHealth(ctx context.Context) ([]domain.ComponentHealth, error) {
	return []domain.ComponentHealth{}, nil
}
