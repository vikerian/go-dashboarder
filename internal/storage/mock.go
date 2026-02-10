package storage

import (
	"context"
	"log/slog"

	"github.com/vikerian/go-dashboarder/internal/domain"
)

type MockStorage struct{}

func NewMockStorage() *MockStorage {
	return &MockStorage{}
}

func (m *MockStorage) SaveIoT(ctx context.Context, data domain.IoTData) error {
	slog.Info("[MOCK DB] Saving IoT Data", "metric", data.Metric, "val", data.Value)
	return nil
}

func (m *MockStorage) SaveWeb(ctx context.Context, data domain.WebData) error {
	slog.Info("[MOCK DB] Saving Web Data", "title", data.Title)
	return nil
}

func (m *MockStorage) Close() error {
	return nil
}
