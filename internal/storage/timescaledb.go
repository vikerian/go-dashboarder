package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
)

type TimescaleStorage struct {
	pool *pgxpool.Pool
}

func NewTimescaleStorage(cfg config.StorageConfig) (*TimescaleStorage, error) {
	// Sestavení connection stringu
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return &TimescaleStorage{pool: pool}, nil
}

func (s *TimescaleStorage) SaveIoT(ctx context.Context, data domain.IoTData) error {
	// Payload pro IoT data (hodnota + jednotka)
	payload := map[string]interface{}{
		"value": data.Value,
		"unit":  data.Unit,
	}
	payloadBytes, _ := json.Marshal(payload)

	query := `INSERT INTO events (ts, topic, source, kind, key, payload)
              VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.pool.Exec(ctx, query,
		data.Timestamp,
		"/data/iot/"+data.Metric,
		data.DeviceID,
		"iot",
		data.Metric,
		payloadBytes,
	)
	return err
}

func (s *TimescaleStorage) SaveWeb(ctx context.Context, data domain.WebData) error {
	// Web data už mají payload (Metadata) jako json.RawMessage
	query := `INSERT INTO events (ts, topic, source, kind, key, payload)
              VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.pool.Exec(ctx, query,
		data.FetchedAt,
		"/data/web",
		data.URL,
		"web",
		data.Title,
		data.Metadata,
	)
	return err
}

func (s *TimescaleStorage) Close() error {
	s.pool.Close()
	return nil
}
