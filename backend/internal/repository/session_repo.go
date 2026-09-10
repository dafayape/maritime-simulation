package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// SessionRepository persists simulation_sessions (SRS §2A.3).
type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, s *domain.Session) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO simulation_sessions
			(session_name, spreading_factor, tx_power_dbm, weather_severity)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_active, created_at`,
		s.SessionName, s.SpreadingFactor, s.TxPowerDbm, s.WeatherSeverity,
	).Scan(&s.ID, &s.IsActive, &s.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, session_name, spreading_factor, tx_power_dbm, weather_severity,
		       is_active, stats_snapshot, created_at, ended_at
		FROM simulation_sessions WHERE id = $1`, id)
	return scanSession(row)
}

func (r *SessionRepository) List(ctx context.Context, limit int) ([]*domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_name, spreading_factor, tx_power_dbm, weather_severity,
		       is_active, stats_snapshot, created_at, ended_at
		FROM simulation_sessions
		ORDER BY created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// ListActive is used on startup to resume the topology scheduler for
// sessions that were live before a restart (VPS resilience).
func (r *SessionRepository) ListActive(ctx context.Context) ([]*domain.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, session_name, spreading_factor, tx_power_dbm, weather_severity,
		       is_active, stats_snapshot, created_at, ended_at
		FROM simulation_sessions
		WHERE is_active = TRUE
		ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *SessionRepository) UpdateWeather(ctx context.Context, id string, severity float64) error {
	return r.updateActive(ctx, id,
		`UPDATE simulation_sessions SET weather_severity = $2 WHERE id = $1 AND is_active = TRUE`,
		severity)
}

func (r *SessionRepository) UpdateRadioParams(ctx context.Context, id string, sf, txPowerDbm int) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE simulation_sessions
		SET spreading_factor = $2, tx_power_dbm = $3
		WHERE id = $1 AND is_active = TRUE`, id, sf, txPowerDbm)
	if err != nil {
		return fmt.Errorf("update radio params %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionInactive
	}
	return nil
}

// Stop deactivates the session and freezes the final statistics snapshot.
func (r *SessionRepository) Stop(ctx context.Context, id string, stats map[string]int64) error {
	statsJSON, err := json.Marshal(stats)
	if err != nil {
		return fmt.Errorf("marshal stats snapshot: %w", err)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE simulation_sessions
		SET is_active = FALSE, ended_at = now(), stats_snapshot = $2::jsonb
		WHERE id = $1 AND is_active = TRUE`, id, string(statsJSON))
	if err != nil {
		return fmt.Errorf("stop session %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionInactive
	}
	return nil
}

// Delete permanently removes a session's history row. dynamic_schemas and
// telemetry_logs cascade via their FKs (ON DELETE CASCADE, 0001_init.sql),
// so this is the single point of truth for wiping a session's audit trail.
// Callers must ensure the session is already stopped (SessionService.Delete
// enforces this) — deleting a still-active session out from under connected
// sockets is never allowed.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM simulation_sessions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete session %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *SessionRepository) updateActive(ctx context.Context, id, sql string, args ...any) error {
	tag, err := r.pool.Exec(ctx, sql, append([]any{id}, args...)...)
	if err != nil {
		return fmt.Errorf("update session %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrSessionInactive
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(row rowScanner) (*domain.Session, error) {
	var s domain.Session
	var statsRaw []byte
	err := row.Scan(&s.ID, &s.SessionName, &s.SpreadingFactor, &s.TxPowerDbm,
		&s.WeatherSeverity, &s.IsActive, &statsRaw, &s.CreatedAt, &s.EndedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	if len(statsRaw) > 0 {
		if err := json.Unmarshal(statsRaw, &s.StatsSnapshot); err != nil {
			return nil, fmt.Errorf("corrupt stats snapshot: %w", err)
		}
	}
	return &s, nil
}
