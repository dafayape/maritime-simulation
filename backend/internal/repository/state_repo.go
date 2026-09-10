package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// dedupTTL is mandated by SRS §2B.2: retried duplicates arrive within
// seconds, and the cache must not grow unbounded.
const dedupTTL = 30 * time.Second

// ackSenderTTL comfortably covers the worst case wait: SF12 timeout (10 s) ×
// 3 retries plus scheduling slack.
const ackSenderTTL = 2 * time.Minute

// SessionParams is the hot-path snapshot of a session's radio configuration,
// cached in Redis so the packet pipeline never touches PostgreSQL.
type SessionParams struct {
	SpreadingFactor int
	TxPowerDbm      int
	WeatherSeverity float64
}

// StateRepository owns the volatile per-session simulation state: parameter
// cache, packet deduplication, ACK back-routing, GPS rate limiting and the
// live statistic counters.
type StateRepository struct {
	rdb *redis.Client
}

func NewStateRepository(rdb *redis.Client) *StateRepository {
	return &StateRepository{rdb: rdb}
}

// --- Session parameter cache -------------------------------------------------

func (r *StateRepository) SaveParams(ctx context.Context, sessionID string, p SessionParams) error {
	err := r.rdb.HSet(ctx, keyParams(sessionID), map[string]any{
		"spreading_factor": p.SpreadingFactor,
		"tx_power_dbm":     p.TxPowerDbm,
		"weather_severity": p.WeatherSeverity,
	}).Err()
	if err != nil {
		return fmt.Errorf("save params %s: %w", sessionID, err)
	}
	return nil
}

func (r *StateRepository) GetParams(ctx context.Context, sessionID string) (SessionParams, error) {
	vals, err := r.rdb.HGetAll(ctx, keyParams(sessionID)).Result()
	if err != nil {
		return SessionParams{}, fmt.Errorf("get params %s: %w", sessionID, err)
	}
	if len(vals) == 0 {
		return SessionParams{}, domain.ErrNotFound
	}
	sf, err := strconv.Atoi(vals["spreading_factor"])
	if err != nil {
		return SessionParams{}, fmt.Errorf("corrupt spreading_factor for %s: %w", sessionID, err)
	}
	tx, err := strconv.Atoi(vals["tx_power_dbm"])
	if err != nil {
		return SessionParams{}, fmt.Errorf("corrupt tx_power_dbm for %s: %w", sessionID, err)
	}
	ws, err := strconv.ParseFloat(vals["weather_severity"], 64)
	if err != nil {
		return SessionParams{}, fmt.Errorf("corrupt weather_severity for %s: %w", sessionID, err)
	}
	return SessionParams{SpreadingFactor: sf, TxPowerDbm: tx, WeatherSeverity: ws}, nil
}

// --- Packet deduplication (SETNX, thread-safe across goroutines) -------------

// MarkPacketSeen returns true when this (packet, receiver) pair is new, and
// false for a retry duplicate. SETNX makes the check-and-set atomic even under
// a broadcast storm (SRS §6.2).
func (r *StateRepository) MarkPacketSeen(ctx context.Context, sessionID, packetID, receiver string) (bool, error) {
	ok, err := r.rdb.SetNX(ctx, keyDedup(sessionID, packetID, receiver), "1", dedupTTL).Result()
	if err != nil {
		return false, fmt.Errorf("dedup setnx %s/%s: %w", sessionID, packetID, err)
	}
	return ok, nil
}

// --- ACK back-routing ---------------------------------------------------------

// RememberAckSender records who put a frame on the air towards receiver so a
// later node:ack can be relayed back.
func (r *StateRepository) RememberAckSender(ctx context.Context, sessionID, packetID, receiver, sender string) error {
	err := r.rdb.Set(ctx, keyAckSender(sessionID, packetID, receiver), sender, ackSenderTTL).Err()
	if err != nil {
		return fmt.Errorf("remember ack sender %s/%s: %w", sessionID, packetID, err)
	}
	return nil
}

// TakeAckSender pops the stored transmitter for (packet, receiver);
// domain.ErrNotFound when the window expired or nothing was forwarded.
func (r *StateRepository) TakeAckSender(ctx context.Context, sessionID, packetID, receiver string) (string, error) {
	sender, err := r.rdb.GetDel(ctx, keyAckSender(sessionID, packetID, receiver)).Result()
	if err == redis.Nil {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("take ack sender %s/%s: %w", sessionID, packetID, err)
	}
	return sender, nil
}

// --- GPS ping rate limiting (PRD cross-cutting concern) ----------------------

// AllowPing enforces at most one geo write per node per interval.
func (r *StateRepository) AllowPing(ctx context.Context, sessionID, nodeID string, interval time.Duration) (bool, error) {
	ok, err := r.rdb.SetNX(ctx, keyPingLimit(sessionID, nodeID), "1", interval).Result()
	if err != nil {
		return false, fmt.Errorf("ping limit %s/%s: %w", sessionID, nodeID, err)
	}
	return ok, nil
}

// --- Live statistics counters -------------------------------------------------

// IncrStat bumps one named counter (HINCRBY is atomic server-side).
func (r *StateRepository) IncrStat(ctx context.Context, sessionID, field string) error {
	if err := r.rdb.HIncrBy(ctx, keyStats(sessionID), field, 1).Err(); err != nil {
		return fmt.Errorf("incr stat %s/%s: %w", sessionID, field, err)
	}
	return nil
}

// GetStats returns every counter of a session.
func (r *StateRepository) GetStats(ctx context.Context, sessionID string) (map[string]int64, error) {
	vals, err := r.rdb.HGetAll(ctx, keyStats(sessionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get stats %s: %w", sessionID, err)
	}
	stats := make(map[string]int64, len(vals))
	for k, v := range vals {
		n, convErr := strconv.ParseInt(v, 10, 64)
		if convErr != nil {
			continue
		}
		stats[k] = n
	}
	return stats, nil
}

// ClearSession removes all volatile keys of a stopped session. Dedup and
// rate-limit keys expire on their own TTLs.
func (r *StateRepository) ClearSession(ctx context.Context, sessionID string) error {
	return r.rdb.Del(ctx,
		keyParams(sessionID),
		keyStats(sessionID),
	).Err()
}
