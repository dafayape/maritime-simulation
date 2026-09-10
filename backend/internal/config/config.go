// Package config loads all runtime configuration from environment variables
// (12-factor style) so the same binary runs unchanged in docker-compose,
// a VPS systemd unit, or a bare `go run`.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every tunable of the simulator backend.
type Config struct {
	// HTTPAddr is the listen address of the REST + WebSocket server.
	HTTPAddr string
	// PprofAddr enables the pprof debug listener when non-empty
	// (e.g. "127.0.0.1:6060"). Keep it bound to localhost in production.
	PprofAddr string

	PostgresDSN string
	RedisAddr   string
	RedisPass   string
	RedisDB     int

	LogLevel string // debug | info | warn | error

	// TopologyInterval is how often MeshTopologyService recomputes routes.
	TopologyInterval time.Duration
	// PingMinInterval rate-limits node GPS updates (1 update / N seconds / node).
	PingMinInterval time.Duration
	// ShutdownTimeout bounds graceful HTTP shutdown.
	ShutdownTimeout time.Duration

	// WorkerPoolSize is the number of goroutines processing inbound WS events.
	// 0 means "auto" (4 × NumCPU).
	WorkerPoolSize int
	// WSSendBuffer is the per-client outbound channel capacity.
	WSSendBuffer int

	// CORSAllowedOrigins is a comma-separated origin list, or "*".
	CORSAllowedOrigins string
}

// Load reads the environment and applies defaults suited to docker-compose.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		PprofAddr:          getEnv("PPROF_ADDR", ""),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:          getEnv("REDIS_PASSWORD", ""),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", "*"),
	}

	cfg.PostgresDSN = os.Getenv("POSTGRES_DSN")
	if cfg.PostgresDSN == "" {
		host := getEnv("POSTGRES_HOST", "localhost")
		port := getEnv("POSTGRES_PORT", "5432")
		user := getEnv("POSTGRES_USER", "simulator")
		pass := getEnv("POSTGRES_PASSWORD", "simulator")
		db := getEnv("POSTGRES_DB", "lora_simulator")
		ssl := getEnv("POSTGRES_SSLMODE", "disable")
		cfg.PostgresDSN = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, db, ssl,
		)
	}

	var err error
	if cfg.RedisDB, err = getEnvInt("REDIS_DB", 0); err != nil {
		return nil, err
	}
	if cfg.WorkerPoolSize, err = getEnvInt("WORKER_POOL_SIZE", 0); err != nil {
		return nil, err
	}
	if cfg.WSSendBuffer, err = getEnvInt("WS_SEND_BUFFER", 256); err != nil {
		return nil, err
	}

	topoSec, err := getEnvInt("TOPOLOGY_INTERVAL_SECONDS", 5)
	if err != nil {
		return nil, err
	}
	if topoSec < 1 {
		return nil, fmt.Errorf("TOPOLOGY_INTERVAL_SECONDS must be >= 1, got %d", topoSec)
	}
	cfg.TopologyInterval = time.Duration(topoSec) * time.Second

	pingSec, err := getEnvInt("PING_MIN_INTERVAL_SECONDS", 3)
	if err != nil {
		return nil, err
	}
	cfg.PingMinInterval = time.Duration(pingSec) * time.Second

	shutSec, err := getEnvInt("SHUTDOWN_TIMEOUT_SECONDS", 10)
	if err != nil {
		return nil, err
	}
	cfg.ShutdownTimeout = time.Duration(shutSec) * time.Second

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("env %s must be an integer, got %q: %w", key, v, err)
	}
	return n, nil
}
