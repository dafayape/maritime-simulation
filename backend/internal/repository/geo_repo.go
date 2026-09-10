package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// GeoRepository tracks live ship coordinates per session using Redis
// geospatial commands (SRS §2B.1).
type GeoRepository struct {
	rdb *redis.Client
}

func NewGeoRepository(rdb *redis.Client) *GeoRepository {
	return &GeoRepository{rdb: rdb}
}

// UpsertPosition stores/refreshes one node coordinate (GEOADD).
func (r *GeoRepository) UpsertPosition(ctx context.Context, sessionID, nodeID string, lat, lng float64) error {
	err := r.rdb.GeoAdd(ctx, keyGeo(sessionID), &redis.GeoLocation{
		Name:      nodeID,
		Latitude:  lat,
		Longitude: lng,
	}).Err()
	if err != nil {
		return fmt.Errorf("geoadd %s/%s: %w", sessionID, nodeID, err)
	}
	return nil
}

// RemoveNode drops a disconnected ship from the spatial index.
func (r *GeoRepository) RemoveNode(ctx context.Context, sessionID, nodeID string) error {
	if err := r.rdb.ZRem(ctx, keyGeo(sessionID), nodeID).Err(); err != nil {
		return fmt.Errorf("zrem geo %s/%s: %w", sessionID, nodeID, err)
	}
	return nil
}

// Position returns one node's coordinate, or domain.ErrNotFound when the
// node has never pinged.
func (r *GeoRepository) Position(ctx context.Context, sessionID, nodeID string) (domain.NodePosition, error) {
	pos, err := r.rdb.GeoPos(ctx, keyGeo(sessionID), nodeID).Result()
	if err != nil {
		return domain.NodePosition{}, fmt.Errorf("geopos %s/%s: %w", sessionID, nodeID, err)
	}
	if len(pos) == 0 || pos[0] == nil {
		return domain.NodePosition{}, domain.ErrNotFound
	}
	return domain.NodePosition{ID: nodeID, Lat: pos[0].Latitude, Lng: pos[0].Longitude}, nil
}

// AllPositions returns every tracked node with its coordinate.
func (r *GeoRepository) AllPositions(ctx context.Context, sessionID string) ([]domain.NodePosition, error) {
	key := keyGeo(sessionID)
	members, err := r.rdb.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("zrange geo %s: %w", sessionID, err)
	}
	if len(members) == 0 {
		return nil, nil
	}
	coords, err := r.rdb.GeoPos(ctx, key, members...).Result()
	if err != nil {
		return nil, fmt.Errorf("geopos batch %s: %w", sessionID, err)
	}
	nodes := make([]domain.NodePosition, 0, len(members))
	for i, m := range members {
		if i < len(coords) && coords[i] != nil {
			nodes = append(nodes, domain.NodePosition{
				ID:  m,
				Lat: coords[i].Latitude,
				Lng: coords[i].Longitude,
			})
		}
	}
	return nodes, nil
}

// Clear removes the whole spatial index of a session (session stop cleanup).
func (r *GeoRepository) Clear(ctx context.Context, sessionID string) error {
	return r.rdb.Del(ctx, keyGeo(sessionID)).Err()
}
