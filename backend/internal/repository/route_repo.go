package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// RouteRepository caches the routing table computed by MeshTopologyService
// (SRS §2B.3). Two hashes are kept per session: the plain node->parent map
// mandated by the SRS, and an enriched JSON variant for diffing/dashboard.
type RouteRepository struct {
	rdb *redis.Client
}

func NewRouteRepository(rdb *redis.Client) *RouteRepository {
	return &RouteRepository{rdb: rdb}
}

// SaveTable atomically replaces the routing table of a session.
func (r *RouteRepository) SaveTable(ctx context.Context, sessionID string, table map[string]domain.RouteEntry) error {
	plain := make(map[string]any, len(table))
	full := make(map[string]any, len(table))
	for nodeID, entry := range table {
		plain[nodeID] = entry.Parent
		raw, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal route %s: %w", nodeID, err)
		}
		full[nodeID] = string(raw)
	}

	pipe := r.rdb.TxPipeline()
	pipe.Del(ctx, keyRoutes(sessionID), keyRoutesFull(sessionID))
	if len(plain) > 0 {
		pipe.HSet(ctx, keyRoutes(sessionID), plain)
		pipe.HSet(ctx, keyRoutesFull(sessionID), full)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("save routes %s: %w", sessionID, err)
	}
	return nil
}

// GetTable returns the enriched routing table, empty when none computed yet.
func (r *RouteRepository) GetTable(ctx context.Context, sessionID string) (map[string]domain.RouteEntry, error) {
	vals, err := r.rdb.HGetAll(ctx, keyRoutesFull(sessionID)).Result()
	if err != nil {
		return nil, fmt.Errorf("get routes %s: %w", sessionID, err)
	}
	table := make(map[string]domain.RouteEntry, len(vals))
	for nodeID, raw := range vals {
		var entry domain.RouteEntry
		if err := json.Unmarshal([]byte(raw), &entry); err != nil {
			return nil, fmt.Errorf("corrupt route entry %s/%s: %w", sessionID, nodeID, err)
		}
		table[nodeID] = entry
	}
	return table, nil
}

// GetRoute returns a single node's route entry, or domain.ErrNotFound.
func (r *RouteRepository) GetRoute(ctx context.Context, sessionID, nodeID string) (domain.RouteEntry, error) {
	raw, err := r.rdb.HGet(ctx, keyRoutesFull(sessionID), nodeID).Result()
	if err == redis.Nil {
		return domain.RouteEntry{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RouteEntry{}, fmt.Errorf("get route %s/%s: %w", sessionID, nodeID, err)
	}
	var entry domain.RouteEntry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return domain.RouteEntry{}, fmt.Errorf("corrupt route entry %s/%s: %w", sessionID, nodeID, err)
	}
	return entry, nil
}

// Clear removes both routing hashes (session stop cleanup).
func (r *RouteRepository) Clear(ctx context.Context, sessionID string) error {
	return r.rdb.Del(ctx, keyRoutes(sessionID), keyRoutesFull(sessionID)).Err()
}
