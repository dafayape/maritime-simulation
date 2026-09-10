package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/service"
)

// Server bundles every dependency the REST handlers need.
type Server struct {
	sessions  *service.SessionService
	schemas   *service.DynamicSchemaService
	topology  *service.MeshTopologyService
	edges     *repository.EdgeRepository
	telemetry *repository.TelemetryRepository
	readiness func(ctx context.Context) error
	log       *slog.Logger
}

func NewServer(
	sessions *service.SessionService,
	schemas *service.DynamicSchemaService,
	topology *service.MeshTopologyService,
	edges *repository.EdgeRepository,
	telemetry *repository.TelemetryRepository,
	readiness func(ctx context.Context) error,
	log *slog.Logger,
) *Server {
	return &Server{
		sessions:  sessions,
		schemas:   schemas,
		topology:  topology,
		edges:     edges,
		telemetry: telemetry,
		readiness: readiness,
		log:       log,
	}
}

// sessionID validates the {id} path parameter as a UUID before it ever
// reaches PostgreSQL.
func sessionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if _, err := uuid.Parse(id); err != nil {
		respondError(w, http.StatusBadRequest, "session id must be a UUID")
		return "", false
	}
	return id, true
}

// --- simulations ---------------------------------------------------------------

type createSimulationRequest struct {
	SessionName     string   `json:"session_name"`
	SpreadingFactor *int     `json:"spreading_factor"`
	TxPowerDbm      *int     `json:"tx_power_dbm"`
	WeatherSeverity *float64 `json:"weather_severity"`
}

// handleCreateSimulation implements POST /api/v1/simulations (SRS §3.1).
func (s *Server) handleCreateSimulation(w http.ResponseWriter, r *http.Request) {
	var req createSimulationRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.SessionName == "" || len(req.SessionName) > 150 {
		respondError(w, http.StatusBadRequest, "session_name is required (max 150 chars)")
		return
	}

	sf, tx, weather := 7, 20, 1.0
	if req.SpreadingFactor != nil {
		sf = *req.SpreadingFactor
	}
	if req.TxPowerDbm != nil {
		tx = *req.TxPowerDbm
	}
	if req.WeatherSeverity != nil {
		weather = *req.WeatherSeverity
	}

	sess, err := s.sessions.Create(r.Context(), req.SessionName, sf, tx, weather)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondCreated(w, "simulation session created", map[string]any{
		"session_id": sess.ID,
		"session":    sess,
	})
}

func (s *Server) handleListSimulations(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50, 1, 200)
	sessions, err := s.sessions.List(r.Context(), limit)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "simulation sessions", sessions)
}

func (s *Server) handleGetSimulation(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	sess, liveStats, err := s.sessions.Get(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "simulation session", map[string]any{
		"session":    sess,
		"live_stats": liveStats,
	})
}

func (s *Server) handleStopSimulation(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	sess, err := s.sessions.Stop(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "simulation session stopped", sess)
}

// handleDeleteSession is DELETE /api/v1/simulations/{id} — permanently wipes
// a stopped session's history (session row + cascaded schema/telemetry).
// Deleting an active session is rejected (409); stop it first.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	if err := s.sessions.Delete(r.Context(), id); err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "simulation session removed", map[string]any{"session_id": id})
}

type updateWeatherRequest struct {
	WeatherSeverity *float64 `json:"weather_severity"`
}

// handleUpdateWeather implements PUT /api/v1/simulations/{id}/weather
// (SRS §3.3 — the Artificial Packet Loss trigger).
func (s *Server) handleUpdateWeather(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	var req updateWeatherRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.WeatherSeverity == nil {
		respondError(w, http.StatusBadRequest, "weather_severity is required")
		return
	}
	sess, err := s.sessions.UpdateWeather(r.Context(), id, *req.WeatherSeverity)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "weather severity updated", sess)
}

type updateParamsRequest struct {
	SpreadingFactor *int `json:"spreading_factor"`
	TxPowerDbm      *int `json:"tx_power_dbm"`
}

// handleUpdateParams implements PUT /api/v1/simulations/{id}/params — the
// runtime SF / TX-power switch demanded by the PRD acceptance criteria
// ("perubahan parameter SF dari REST API langsung memicu validasi batas
// byte secara real-time").
func (s *Server) handleUpdateParams(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	var req updateParamsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.SpreadingFactor == nil && req.TxPowerDbm == nil {
		respondError(w, http.StatusBadRequest, "provide spreading_factor and/or tx_power_dbm")
		return
	}
	sess, err := s.sessions.UpdateRadioParams(r.Context(), id, req.SpreadingFactor, req.TxPowerDbm)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "radio parameters updated", sess)
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	stats, err := s.sessions.LiveStats(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "live simulation statistics", stats)
}

// --- dynamic schema --------------------------------------------------------------

type setSchemaRequest struct {
	Fields []domain.SchemaField `json:"fields"`
}

// handleSetSchema implements POST /api/v1/simulations/{id}/schema (SRS §3.2).
func (s *Server) handleSetSchema(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	var req setSchemaRequest
	if !decodeBody(w, r, &req) {
		return
	}
	schema, estimate, warning, err := s.schemas.SetSchema(r.Context(), id, req.Fields)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "dynamic schema stored and pushed to active nodes", map[string]any{
		"schema":                  schema,
		"estimated_payload_bytes": estimate,
		"warning":                 warning,
	})
}

func (s *Server) handleGetSchema(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	schema, err := s.schemas.GetActive(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "active dynamic schema", schema)
}

// --- topology & telemetry ----------------------------------------------------------

func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	snap, err := s.topology.Snapshot(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "current mesh topology", snap)
}

func (s *Server) handleListTelemetry(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(w, r)
	if !ok {
		return
	}
	limit := queryInt(r, "limit", 50, 1, 500)
	offset := queryInt(r, "offset", 0, 0, 1_000_000)

	items, err := s.telemetry.ListBySession(r.Context(), id, limit, offset)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	total, err := s.telemetry.CountBySession(r.Context(), id)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "telemetry logs", map[string]any{
		"items":  items,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// --- edges ---------------------------------------------------------------------------

type createEdgeRequest struct {
	EdgeCode  string   `json:"edge_code"`
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func (s *Server) handleCreateEdge(w http.ResponseWriter, r *http.Request) {
	var req createEdgeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	switch {
	case req.EdgeCode == "" || len(req.EdgeCode) > 50:
		respondError(w, http.StatusBadRequest, "edge_code is required (max 50 chars)")
		return
	case req.Name == "" || len(req.Name) > 150:
		respondError(w, http.StatusBadRequest, "name is required (max 150 chars)")
		return
	case req.Latitude == nil || *req.Latitude < -90 || *req.Latitude > 90:
		respondError(w, http.StatusBadRequest, "latitude is required and must be within -90..90")
		return
	case req.Longitude == nil || *req.Longitude < -180 || *req.Longitude > 180:
		respondError(w, http.StatusBadRequest, "longitude is required and must be within -180..180")
		return
	}

	edge := &domain.Edge{
		EdgeCode:  req.EdgeCode,
		Name:      req.Name,
		Latitude:  *req.Latitude,
		Longitude: *req.Longitude,
	}
	if err := s.edges.Create(r.Context(), edge); err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondCreated(w, "virtual edge registered", edge)
}

type updateEdgeRequest struct {
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

// handleUpdateEdge is PUT /api/v1/edges/{code} — edits name/coordinates in
// place. edge_code is immutable (it is the routing key ships reference as
// target_parent), so existing routes and in-flight sessions are unaffected;
// this is the safe alternative to delete+recreate when a Syahbandar just
// needs a corrected name or position.
func (s *Server) handleUpdateEdge(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" || len(code) > 50 {
		respondError(w, http.StatusBadRequest, "edge_code is required (max 50 chars)")
		return
	}
	var req updateEdgeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	switch {
	case req.Name == "" || len(req.Name) > 150:
		respondError(w, http.StatusBadRequest, "name is required (max 150 chars)")
		return
	case req.Latitude == nil || *req.Latitude < -90 || *req.Latitude > 90:
		respondError(w, http.StatusBadRequest, "latitude is required and must be within -90..90")
		return
	case req.Longitude == nil || *req.Longitude < -180 || *req.Longitude > 180:
		respondError(w, http.StatusBadRequest, "longitude is required and must be within -180..180")
		return
	}

	edge, err := s.edges.Update(r.Context(), code, req.Name, *req.Latitude, *req.Longitude)
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "virtual edge updated", edge)
}

func (s *Server) handleListEdges(w http.ResponseWriter, r *http.Request) {
	edges, err := s.edges.List(r.Context())
	if err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "virtual edges", edges)
}

func (s *Server) handleDeleteEdge(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" || len(code) > 50 {
		respondError(w, http.StatusBadRequest, "edge_code is required (max 50 chars)")
		return
	}
	if err := s.edges.Delete(r.Context(), code); err != nil {
		respondDomainError(w, s.log, err)
		return
	}
	respondOK(w, "virtual edge removed", map[string]any{"edge_code": code})
}

// --- health -----------------------------------------------------------------------

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	respondOK(w, "alive", map[string]any{"time": time.Now().UTC()})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := s.readiness(ctx); err != nil {
		respondError(w, http.StatusServiceUnavailable, "not ready: "+err.Error())
		return
	}
	respondOK(w, "ready", nil)
}

// --- helpers -----------------------------------------------------------------------

func queryInt(r *http.Request, name string, def, min, max int) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < min {
		return def
	}
	if n > max {
		return max
	}
	return n
}
