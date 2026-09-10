package service

import (
	"context"
	"log/slog"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/protocol"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

// SessionService owns the simulation session lifecycle: creation, live
// parameter changes (weather / SF / TX power), stop with statistics
// snapshot, and resume-after-restart.
type SessionService struct {
	sessions *repository.SessionRepository
	state    *repository.StateRepository
	geo      *repository.GeoRepository
	routes   *repository.RouteRepository
	topology *MeshTopologyService
	bcast    Broadcaster
	log      *slog.Logger
}

func NewSessionService(
	sessions *repository.SessionRepository,
	state *repository.StateRepository,
	geo *repository.GeoRepository,
	routes *repository.RouteRepository,
	topology *MeshTopologyService,
	bcast Broadcaster,
	log *slog.Logger,
) *SessionService {
	return &SessionService{
		sessions: sessions,
		state:    state,
		geo:      geo,
		routes:   routes,
		topology: topology,
		bcast:    bcast,
		log:      log,
	}
}

// EnvParamsFrom derives the full env:sync_params payload from the cached
// radio parameters, including the Data Link values clients must obey.
func EnvParamsFrom(p repository.SessionParams) protocol.EnvSyncParams {
	return protocol.EnvSyncParams{
		Event:           protocol.EvEnvSyncParams,
		SpreadingFactor: p.SpreadingFactor,
		MaxPayloadBytes: domain.MaxPayloadBytes(p.SpreadingFactor),
		AckTimeoutMs:    domain.AckTimeout(p.SpreadingFactor).Milliseconds(),
		MaxRetries:      domain.MaxRetries,
		TxPowerDbm:      p.TxPowerDbm,
		MaxRangeKm:      domain.MaxRangeKm(p.TxPowerDbm),
		WeatherSeverity: p.WeatherSeverity,
	}
}

// Create validates and starts a new simulation session.
func (s *SessionService) Create(ctx context.Context, name string, sf, txPowerDbm int, weather float64) (*domain.Session, error) {
	if err := domain.ValidateSpreadingFactor(sf); err != nil {
		return nil, err
	}
	if err := domain.ValidateTxPower(txPowerDbm); err != nil {
		return nil, err
	}
	if err := domain.ValidateWeatherSeverity(weather); err != nil {
		return nil, err
	}

	sess := &domain.Session{
		SessionName:     name,
		SpreadingFactor: sf,
		TxPowerDbm:      txPowerDbm,
		WeatherSeverity: weather,
	}
	if err := s.sessions.Create(ctx, sess); err != nil {
		return nil, err
	}
	if err := s.cacheParams(ctx, sess); err != nil {
		return nil, err
	}
	s.topology.AddSession(sess.ID)

	s.log.Info("simulation session started",
		"session_id", sess.ID, "name", name, "sf", sf, "tx_power_dbm", txPowerDbm, "weather", weather)
	return sess, nil
}

// UpdateWeather changes the packet-loss multiplier mid-flight (SRS §3.3) and
// syncs every connected client immediately.
func (s *SessionService) UpdateWeather(ctx context.Context, id string, severity float64) (*domain.Session, error) {
	if err := domain.ValidateWeatherSeverity(severity); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateWeather(ctx, id, severity); err != nil {
		return nil, err
	}
	sess, err := s.refreshAndBroadcast(ctx, id)
	if err != nil {
		return nil, err
	}
	s.log.Info("weather severity updated", "session_id", id, "weather_severity", severity)
	return sess, nil
}

// UpdateRadioParams changes SF and/or TX power at runtime. Passing nil keeps
// the current value. Byte-limit validation of in-flight packets picks the new
// SF up on the very next frame because the engine reads the Redis cache.
func (s *SessionService) UpdateRadioParams(ctx context.Context, id string, sf, txPowerDbm *int) (*domain.Session, error) {
	current, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !current.IsActive {
		return nil, domain.ErrSessionInactive
	}

	newSF := current.SpreadingFactor
	if sf != nil {
		newSF = *sf
	}
	newTx := current.TxPowerDbm
	if txPowerDbm != nil {
		newTx = *txPowerDbm
	}
	if err := domain.ValidateSpreadingFactor(newSF); err != nil {
		return nil, err
	}
	if err := domain.ValidateTxPower(newTx); err != nil {
		return nil, err
	}
	if err := s.sessions.UpdateRadioParams(ctx, id, newSF, newTx); err != nil {
		return nil, err
	}

	sess, err := s.refreshAndBroadcast(ctx, id)
	if err != nil {
		return nil, err
	}
	s.log.Info("radio params updated", "session_id", id, "sf", newSF, "tx_power_dbm", newTx)
	return sess, nil
}

// Stop deactivates a session: snapshots the final statistics into
// PostgreSQL, notifies and disconnects every socket, and clears the
// volatile Redis state.
func (s *SessionService) Stop(ctx context.Context, id string) (*domain.Session, error) {
	stats, err := s.state.GetStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.Stop(ctx, id, stats); err != nil {
		return nil, err
	}

	ended := protocol.SessionEnded{
		Event:     protocol.EvSessionEnded,
		SessionID: id,
		Message:   "simulation session stopped by dashboard",
	}
	s.bcast.BroadcastToNodes(id, ended)
	s.bcast.BroadcastToMonitors(id, ended)
	s.bcast.KickSession(id)
	s.topology.RemoveSession(id)

	// Volatile state cleanup; dedup/rate-limit keys expire via TTL.
	if err := s.geo.Clear(ctx, id); err != nil {
		s.log.Warn("geo cleanup failed", "session_id", id, "error", err.Error())
	}
	if err := s.routes.Clear(ctx, id); err != nil {
		s.log.Warn("routes cleanup failed", "session_id", id, "error", err.Error())
	}
	if err := s.state.ClearSession(ctx, id); err != nil {
		s.log.Warn("state cleanup failed", "session_id", id, "error", err.Error())
	}

	s.log.Info("simulation session stopped", "session_id", id, "stats", stats)
	return s.sessions.GetByID(ctx, id)
}

// Get returns a session with live statistics merged in while it is active.
func (s *SessionService) Get(ctx context.Context, id string) (*domain.Session, map[string]int64, error) {
	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	var live map[string]int64
	if sess.IsActive {
		if live, err = s.state.GetStats(ctx, id); err != nil {
			return nil, nil, err
		}
	}
	return sess, live, nil
}

func (s *SessionService) List(ctx context.Context, limit int) ([]*domain.Session, error) {
	return s.sessions.List(ctx, limit)
}

// Delete permanently removes a session's history (session row + cascaded
// dynamic_schemas/telemetry_logs). Only stopped sessions may be deleted —
// an active one must go through Stop first, so connected sockets are never
// yanked out from under a live simulation just because its DB row vanished.
func (s *SessionService) Delete(ctx context.Context, id string) error {
	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if sess.IsActive {
		return domain.ErrSessionStillActive
	}
	if err := s.sessions.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("simulation session history deleted", "session_id", id)
	return nil
}

// LiveStats exposes the Redis counters for the stats endpoint.
func (s *SessionService) LiveStats(ctx context.Context, id string) (map[string]int64, error) {
	if _, err := s.sessions.GetByID(ctx, id); err != nil {
		return nil, err
	}
	return s.state.GetStats(ctx, id)
}

// ResumeActive re-registers every still-active session after a server
// restart so the topology scheduler and parameter cache survive VPS reboots.
func (s *SessionService) ResumeActive(ctx context.Context) error {
	active, err := s.sessions.ListActive(ctx)
	if err != nil {
		return err
	}
	for _, sess := range active {
		if err := s.cacheParams(ctx, sess); err != nil {
			return err
		}
		s.topology.AddSession(sess.ID)
	}
	if len(active) > 0 {
		s.log.Info("resumed active sessions after restart", "count", len(active))
	}
	return nil
}

func (s *SessionService) cacheParams(ctx context.Context, sess *domain.Session) error {
	return s.state.SaveParams(ctx, sess.ID, repository.SessionParams{
		SpreadingFactor: sess.SpreadingFactor,
		TxPowerDbm:      sess.TxPowerDbm,
		WeatherSeverity: sess.WeatherSeverity,
	})
}

func (s *SessionService) refreshAndBroadcast(ctx context.Context, id string) (*domain.Session, error) {
	sess, err := s.sessions.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.cacheParams(ctx, sess); err != nil {
		return nil, err
	}
	env := EnvParamsFrom(repository.SessionParams{
		SpreadingFactor: sess.SpreadingFactor,
		TxPowerDbm:      sess.TxPowerDbm,
		WeatherSeverity: sess.WeatherSeverity,
	})
	s.bcast.BroadcastToNodes(id, env)
	s.bcast.BroadcastToMonitors(id, env)
	return sess, nil
}
