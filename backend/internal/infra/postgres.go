package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool opens a pgx connection pool and waits for the database to
// become reachable. The retry loop covers docker-compose startup ordering and
// VPS reboots where Postgres comes up slower than the app container.
func NewPostgresPool(ctx context.Context, dsn string, log *slog.Logger) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	poolCfg.MaxConns = 16
	poolCfg.MinConns = 2
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	const maxAttempts = 30
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			log.Info("postgres connected", "host", poolCfg.ConnConfig.Host)
			return pool, nil
		}
		if attempt >= maxAttempts {
			pool.Close()
			return nil, fmt.Errorf("postgres unreachable after %d attempts: %w", maxAttempts, err)
		}
		log.Warn("postgres not ready, retrying", "attempt", attempt, "error", err.Error())
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
