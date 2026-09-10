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

// SchemaRepository persists dynamic_schemas (SRS §2A.4). The newest row per
// session is the active payload contract.
type SchemaRepository struct {
	pool *pgxpool.Pool
}

func NewSchemaRepository(pool *pgxpool.Pool) *SchemaRepository {
	return &SchemaRepository{pool: pool}
}

type schemaDefinition struct {
	Fields []domain.SchemaField `json:"fields"`
}

func (r *SchemaRepository) Create(ctx context.Context, sessionID string, fields []domain.SchemaField) (*domain.DynamicSchema, error) {
	raw, err := json.Marshal(schemaDefinition{Fields: fields})
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}

	s := &domain.DynamicSchema{SessionID: sessionID, Fields: fields}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO dynamic_schemas (session_id, schema_definition)
		VALUES ($1, $2::jsonb)
		RETURNING id, created_at`,
		sessionID, string(raw),
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert schema: %w", err)
	}
	return s, nil
}

// GetLatest returns the active schema of a session, or domain.ErrNotFound
// when the dashboard has not injected one yet.
func (r *SchemaRepository) GetLatest(ctx context.Context, sessionID string) (*domain.DynamicSchema, error) {
	var s domain.DynamicSchema
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, session_id, schema_definition, created_at
		FROM dynamic_schemas
		WHERE session_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, sessionID,
	).Scan(&s.ID, &s.SessionID, &raw, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest schema %s: %w", sessionID, err)
	}

	var def schemaDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("corrupt schema definition %d: %w", s.ID, err)
	}
	s.Fields = def.Fields
	return &s, nil
}
