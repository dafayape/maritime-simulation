package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// EdgeRepository persists virtual_edges — the Syahbandar endpoints packets
// must reach (SRS §2A.2).
type EdgeRepository struct {
	pool *pgxpool.Pool
}

func NewEdgeRepository(pool *pgxpool.Pool) *EdgeRepository {
	return &EdgeRepository{pool: pool}
}

func (r *EdgeRepository) Create(ctx context.Context, e *domain.Edge) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO virtual_edges (edge_code, name, latitude, longitude)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		e.EdgeCode, e.Name, e.Latitude, e.Longitude,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: edge_code %q already exists", domain.ErrValidation, e.EdgeCode)
	}
	if err != nil {
		return fmt.Errorf("insert edge: %w", err)
	}
	return nil
}

func (r *EdgeRepository) List(ctx context.Context) ([]domain.Edge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, edge_code, name, latitude, longitude, created_at, updated_at
		FROM virtual_edges ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	defer rows.Close()

	var edges []domain.Edge
	for rows.Next() {
		var e domain.Edge
		if err := rows.Scan(&e.ID, &e.EdgeCode, &e.Name, &e.Latitude, &e.Longitude,
			&e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}

// Update edits a Virtual Edge's name/coordinates in place. edge_code is
// immutable (it is the routing key nodes reference as target_parent), so
// only name/latitude/longitude change — existing routes and telemetry
// history stay valid, unlike Delete. Returns ErrNotFound when no row matched.
func (r *EdgeRepository) Update(ctx context.Context, code, name string, lat, lng float64) (*domain.Edge, error) {
	var e domain.Edge
	err := r.pool.QueryRow(ctx, `
		UPDATE virtual_edges
		SET name = $2, latitude = $3, longitude = $4, updated_at = now()
		WHERE edge_code = $1
		RETURNING id, edge_code, name, latitude, longitude, created_at, updated_at`,
		code, name, lat, lng,
	).Scan(&e.ID, &e.EdgeCode, &e.Name, &e.Latitude, &e.Longitude, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update edge %s: %w", code, err)
	}
	return &e, nil
}

// Delete removes a virtual edge by its code. Telemetry rows that referenced it
// keep their history — the FK virtual_edges(id) is ON DELETE SET NULL — so
// deleting a Syahbandar gateway only affects future routing, never the audit
// trail. Returns ErrNotFound when no row matched.
func (r *EdgeRepository) Delete(ctx context.Context, code string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM virtual_edges WHERE edge_code = $1`, code)
	if err != nil {
		return fmt.Errorf("delete edge %s: %w", code, err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *EdgeRepository) GetByCode(ctx context.Context, code string) (*domain.Edge, error) {
	var e domain.Edge
	err := r.pool.QueryRow(ctx, `
		SELECT id, edge_code, name, latitude, longitude, created_at, updated_at
		FROM virtual_edges WHERE edge_code = $1`, code,
	).Scan(&e.ID, &e.EdgeCode, &e.Name, &e.Latitude, &e.Longitude, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get edge %s: %w", code, err)
	}
	return &e, nil
}
