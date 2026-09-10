package api

import (
	"log/slog"
	"net/http"
)

// WSHandlers are the transport endpoints mounted alongside the REST routes.
type WSHandlers struct {
	Nodes   http.HandlerFunc
	Monitor http.HandlerFunc
}

// Router assembles the full HTTP surface with the middleware chain
// recover -> access log -> CORS.
func Router(s *Server, ws WSHandlers, corsOrigins string, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Simulation lifecycle (SRS §3).
	mux.HandleFunc("POST /api/v1/simulations", s.handleCreateSimulation)
	mux.HandleFunc("GET /api/v1/simulations", s.handleListSimulations)
	mux.HandleFunc("GET /api/v1/simulations/{id}", s.handleGetSimulation)
	mux.HandleFunc("POST /api/v1/simulations/{id}/stop", s.handleStopSimulation)
	mux.HandleFunc("DELETE /api/v1/simulations/{id}", s.handleDeleteSession)
	mux.HandleFunc("PUT /api/v1/simulations/{id}/weather", s.handleUpdateWeather)
	mux.HandleFunc("PUT /api/v1/simulations/{id}/params", s.handleUpdateParams)

	// Dynamic schema.
	mux.HandleFunc("POST /api/v1/simulations/{id}/schema", s.handleSetSchema)
	mux.HandleFunc("GET /api/v1/simulations/{id}/schema", s.handleGetSchema)

	// Dashboard reads.
	mux.HandleFunc("GET /api/v1/simulations/{id}/topology", s.handleGetTopology)
	mux.HandleFunc("GET /api/v1/simulations/{id}/telemetry", s.handleListTelemetry)
	mux.HandleFunc("GET /api/v1/simulations/{id}/stats", s.handleGetStats)

	// Virtual edges (Syahbandar master data).
	mux.HandleFunc("POST /api/v1/edges", s.handleCreateEdge)
	mux.HandleFunc("GET /api/v1/edges", s.handleListEdges)
	mux.HandleFunc("PUT /api/v1/edges/{code}", s.handleUpdateEdge)
	mux.HandleFunc("DELETE /api/v1/edges/{code}", s.handleDeleteEdge)

	// WebSocket surfaces.
	mux.HandleFunc("GET /ws/nodes", ws.Nodes)
	mux.HandleFunc("GET /ws/monitor", ws.Monitor)

	// Health.
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)

	return withRecover(log, withAccessLog(log, withCORS(corsOrigins, mux)))
}
