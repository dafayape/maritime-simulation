package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/protocol"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

// DynamicSchemaService validates, persists and distributes the payload
// structure that the dashboard injects into every mobile node (PRD §3.2).
type DynamicSchemaService struct {
	schemas  *repository.SchemaRepository
	sessions *repository.SessionRepository
	bcast    Broadcaster
	log      *slog.Logger
}

func NewDynamicSchemaService(
	schemas *repository.SchemaRepository,
	sessions *repository.SessionRepository,
	bcast Broadcaster,
	log *slog.Logger,
) *DynamicSchemaService {
	return &DynamicSchemaService{schemas: schemas, sessions: sessions, bcast: bcast, log: log}
}

// SetSchema stores a new schema version and pushes it to every active node
// and monitor of the session. It returns the stored schema plus the
// worst-case MessagePack size estimate, and a non-fatal warning when that
// estimate does not fit the session's current Spreading Factor window.
func (s *DynamicSchemaService) SetSchema(ctx context.Context, sessionID string, fields []domain.SchemaField) (*domain.DynamicSchema, int, string, error) {
	if err := domain.ValidateSchemaFields(fields); err != nil {
		return nil, 0, "", err
	}

	sess, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, 0, "", err
	}
	if !sess.IsActive {
		return nil, 0, "", domain.ErrSessionInactive
	}

	schema, err := s.schemas.Create(ctx, sessionID, fields)
	if err != nil {
		return nil, 0, "", err
	}

	estimate := domain.EstimatePackedBytes(fields)
	warning := ""
	if limit := domain.MaxPayloadBytes(sess.SpreadingFactor); estimate > limit {
		warning = fmt.Sprintf(
			"estimated packed size %d bytes exceeds the SF%d payload limit of %d bytes; full payloads will be dropped",
			estimate, sess.SpreadingFactor, limit,
		)
	}

	sync := protocol.SchemaSync{
		Event:                protocol.EvSchemaSync,
		Fields:               fields,
		EstimatedPackedBytes: estimate,
	}
	s.bcast.BroadcastToNodes(sessionID, sync)
	s.bcast.BroadcastToMonitors(sessionID, sync)

	s.log.Info("dynamic schema updated",
		"session_id", sessionID,
		"schema_id", schema.ID,
		"fields", len(fields),
		"estimated_bytes", estimate,
	)
	return schema, estimate, warning, nil
}

// GetActive returns the latest schema of a session (domain.ErrNotFound when
// none was injected yet).
func (s *DynamicSchemaService) GetActive(ctx context.Context, sessionID string) (*domain.DynamicSchema, error) {
	return s.schemas.GetLatest(ctx, sessionID)
}
