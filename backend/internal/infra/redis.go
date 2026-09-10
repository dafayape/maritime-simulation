package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient connects to Redis and waits for it to answer PING, mirroring
// the Postgres retry behaviour for container orchestration.
func NewRedisClient(ctx context.Context, addr, password string, db int, log *slog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     32,
	})

	const maxAttempts = 30
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := client.Ping(pingCtx).Err()
		cancel()
		if err == nil {
			log.Info("redis connected", "addr", addr)
			return client, nil
		}
		if attempt >= maxAttempts {
			_ = client.Close()
			return nil, fmt.Errorf("redis unreachable after %d attempts: %w", maxAttempts, err)
		}
		log.Warn("redis not ready, retrying", "attempt", attempt, "error", err.Error())
		select {
		case <-ctx.Done():
			_ = client.Close()
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
