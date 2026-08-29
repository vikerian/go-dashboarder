package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vikerian/go-dashboarder/internal/config"
	"github.com/vikerian/go-dashboarder/internal/domain"
)

type TimescaleStorage struct {
	pool *pgxpool.Pool
}

func NewTimescaleStorage(cfg config.StorageConfig) (*TimescaleStorage, error) {

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return &TimescaleStorage{pool: pool}, nil
}

// GetLatestEvents nyní korektně implementuje interface s parametrem kind
func (s *TimescaleStorage) GetLatestEvents(ctx context.Context, kind string, limit int) ([]domain.IoTData, error) {
	results := []domain.IoTData{} // Inicializace prázdného pole (ne nil)

	// SQL Dotaz: Pokud je kind prázdný, bere všechno. Pokud ne, filtruje.
	query := `
		SELECT
			ts,
			source,
			key,
			COALESCE((payload->>'value')::float, 0),
			COALESCE((payload->>'unit'), 'unknown'),
			topic
		FROM events`

	var rowsRows any
	var err error

	if kind != "" {
		query += ` WHERE kind ILIKE $1 ORDER BY ts DESC LIMIT $2`
		rowsRows, err = s.pool.Query(ctx, query, kind, limit)
	} else {
		query += ` ORDER BY ts DESC LIMIT $1`
		rowsRows, err = s.pool.Query(ctx, query, limit)
	}

	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Přetypování na pgx.Rows
	rows := rowsRows.(interface {
		Next() bool
		Scan(dest ...any) error
		Close()
	})
	defer rows.Close()

	for rows.Next() {
		var d domain.IoTData
		if err := rows.Scan(&d.Timestamp, &d.DeviceID, &d.Metric, &d.Value, &d.Unit, &d.Topic); err != nil {
			slog.Warn("Scan row failed", "err", err)
			continue
		}
		results = append(results, d)
	}

	return results, nil
}

// Ostatní metody musí v souboru zůstat, aby to fungovalo:
func (s *TimescaleStorage) SaveIoT(ctx context.Context, data domain.IoTData) error {
	// Připravíme payload jako JSON, jak jsme byli zvyklí
	payload, _ := json.Marshal(map[string]any{
		"value": data.Value,
		"unit":  data.Unit,
	})

	// SQL dotaz, který už má 6 parametrů místo 5
	query := `
			INSERT INTO events (ts, source, kind, key, payload, topic)
			VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.pool.Exec(ctx, query,
		data.Timestamp, // $1
		data.DeviceID,  // $2
		"iot",          // $3 (kind)
		data.Metric,    // $4 (key)
		payload,        // $5
		data.Topic,     // $6 <--- A TADY to posíláme do DB
	)

	if err != nil {
		slog.Error("Failed to save event to DB", "err", err, "topic", data.Topic)
		return err
	}
	return nil
}

func (s *TimescaleStorage) SaveWeb(ctx context.Context, data domain.WebData) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO events (ts, source, kind, key, payload) VALUES ($1, $2, $3, $4, $5)",
		data.FetchedAt, data.URL, "web", data.Title, data.Metadata)
	return err
}

func (s *TimescaleStorage) GetEventsByMetric(ctx context.Context, metric string, hours int) ([]domain.IoTData, error) {
	// Implementace pro grafy podle metriky
	return []domain.IoTData{}, nil
}

func (s *TimescaleStorage) Close() error {
	s.pool.Close()
	return nil
}

// UpdateComponentHealth provede UPSERT do tabulky zdraví
func (s *TimescaleStorage) UpdateComponentHealth(ctx context.Context, name, status, msg string, lastSeen time.Time) error {
	query := `
		INSERT INTO component_health (name, status, last_seen, message)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE
		SET status = EXCLUDED.status,
		    last_seen = EXCLUDED.last_seen,
		    message = EXCLUDED.message`

	_, err := s.pool.Exec(ctx, query, name, status, lastSeen, msg)
	if err != nil {
		return fmt.Errorf("failed to update component health: %w", err)
	}
	return nil
}

// GetComponentHealth vytáhne seznam pro API endpoint
func (s *TimescaleStorage) GetComponentHealth(ctx context.Context) ([]domain.ComponentHealth, error) {
	// Inicializace jako PRÁZDNÉ POLE, ne nil. Tím zmizí ten 'null' v curl.
	results := []domain.ComponentHealth{}

	query := `SELECT name, status, last_seen, message FROM component_health ORDER BY name ASC`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return results, err
	}
	defer rows.Close()

	for rows.Next() {
		var c domain.ComponentHealth
		if err := rows.Scan(&c.Name, &c.Status, &c.LastSeen, &c.Message); err != nil {
			continue
		}
		results = append(results, c)
	}
	return results, nil
}
