package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// TelemetryRepository persists packets that survived the mesh and reached a
// Virtual Edge (SRS §2A.5).
type TelemetryRepository struct {
	pool *pgxpool.Pool
}

func NewTelemetryRepository(pool *pgxpool.Pool) *TelemetryRepository {
	return &TelemetryRepository{pool: pool}
}

func (r *TelemetryRepository) Insert(ctx context.Context, log *domain.TelemetryLog) error {
	payloadJSON, err := json.Marshal(log.DecodedPayload)
	if err != nil {
		return fmt.Errorf("marshal decoded payload: %w", err)
	}
	// edge_code_snapshot freezes the delivering edge's code at write time
	// (0002_edge_lifecycle.sql) so the name survives even if that edge is
	// later deleted from master data — only the FK (edge_id) goes NULL.
	err = r.pool.QueryRow(ctx, `
		INSERT INTO telemetry_logs
			(session_id, origin_node_id, edge_id, edge_code_snapshot, hop_count, routing_path, decoded_payload)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7::jsonb)
		RETURNING id, arrived_at`,
		log.SessionID, log.OriginNodeID, log.EdgeID, log.EdgeCode, log.HopCount, log.RoutingPath, string(payloadJSON),
	).Scan(&log.ID, &log.ArrivedAt)
	if err != nil {
		return fmt.Errorf("insert telemetry: %w", err)
	}
	return nil
}

// ListBySession returns the newest telemetry rows for the dashboard's
// historical report table.
func (r *TelemetryRepository) ListBySession(ctx context.Context, sessionID string, limit, offset int) ([]domain.TelemetryLog, error) {
	// Two LEFT JOINs by design: e_live resolves legacy rows written before
	// 0002_edge_lifecycle.sql (edge_code_snapshot still NULL, edge_id intact)
	// so their name isn't lost; e_active is looked up by CODE (not id) so it
	// still finds a match even after the original edge_id was nulled out by
	// ON DELETE SET NULL — its presence/absence is exactly "edge_active".
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.session_id, t.origin_node_id, t.edge_id,
		       COALESCE(t.edge_code_snapshot, e_live.edge_code, '') AS edge_code,
		       (e_active.id IS NOT NULL) AS edge_active,
		       t.hop_count, COALESCE(t.routing_path, ''), t.decoded_payload, t.arrived_at
		FROM telemetry_logs t
		LEFT JOIN virtual_edges e_live ON e_live.id = t.edge_id
		LEFT JOIN virtual_edges e_active
		       ON e_active.edge_code = COALESCE(t.edge_code_snapshot, e_live.edge_code)
		WHERE t.session_id = $1
		ORDER BY t.arrived_at DESC, t.id DESC
		LIMIT $2 OFFSET $3`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list telemetry %s: %w", sessionID, err)
	}
	defer rows.Close()

	var logs []domain.TelemetryLog
	for rows.Next() {
		var l domain.TelemetryLog
		var payloadRaw []byte
		if err := rows.Scan(&l.ID, &l.SessionID, &l.OriginNodeID, &l.EdgeID,
			&l.EdgeCode, &l.EdgeActive, &l.HopCount, &l.RoutingPath, &payloadRaw, &l.ArrivedAt); err != nil {
			return nil, fmt.Errorf("scan telemetry: %w", err)
		}
		if len(payloadRaw) > 0 {
			if err := json.Unmarshal(payloadRaw, &l.DecodedPayload); err != nil {
				return nil, fmt.Errorf("corrupt telemetry payload %d: %w", l.ID, err)
			}
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// CountBySession supports pagination in the dashboard.
func (r *TelemetryRepository) CountBySession(ctx context.Context, sessionID string) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM telemetry_logs WHERE session_id = $1`, sessionID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count telemetry %s: %w", sessionID, err)
	}
	return count, nil
}
