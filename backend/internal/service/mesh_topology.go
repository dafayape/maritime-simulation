package service

import (
	"context"
	"log/slog"
	"math"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/protocol"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

// distanceJitterKm suppresses routing rebroadcasts caused by tiny GPS drift:
// a route counts as changed only when parent/status/hop change or the parent
// distance moves by more than this.
const distanceJitterKm = 0.05

// MeshTopologyService recomputes the mesh routing DAG for every active
// session on a fixed interval (SRS §5C: every 5 seconds), persists it to
// Redis and dictates each ship's parent over WebSocket (PRD §3.2).
type MeshTopologyService struct {
	geo    *repository.GeoRepository
	routes *repository.RouteRepository
	edges  *repository.EdgeRepository
	state  *repository.StateRepository
	bcast  Broadcaster
	log    *slog.Logger

	interval time.Duration

	mu     sync.RWMutex
	active map[string]bool
}

func NewMeshTopologyService(
	geo *repository.GeoRepository,
	routes *repository.RouteRepository,
	edges *repository.EdgeRepository,
	state *repository.StateRepository,
	bcast Broadcaster,
	interval time.Duration,
	log *slog.Logger,
) *MeshTopologyService {
	return &MeshTopologyService{
		geo:      geo,
		routes:   routes,
		edges:    edges,
		state:    state,
		bcast:    bcast,
		log:      log,
		interval: interval,
		active:   make(map[string]bool),
	}
}

// AddSession registers a session with the scheduler.
func (s *MeshTopologyService) AddSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active[sessionID] = true
}

// RemoveSession unregisters a stopped session.
func (s *MeshTopologyService) RemoveSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, sessionID)
}

func (s *MeshTopologyService) activeSessions() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.active))
	for id := range s.active {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Run is the scheduler goroutine. It exits when ctx is cancelled (graceful
// shutdown) and survives per-session panics so one broken session can never
// kill the whole simulator.
func (s *MeshTopologyService) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("mesh topology scheduler started", "interval", s.interval.String())
	for {
		select {
		case <-ctx.Done():
			s.log.Info("mesh topology scheduler stopped")
			return
		case <-ticker.C:
			for _, sessionID := range s.activeSessions() {
				s.safeRecompute(ctx, sessionID)
			}
		}
	}
}

func (s *MeshTopologyService) safeRecompute(ctx context.Context, sessionID string) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("topology recompute panicked",
				"session_id", sessionID, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	if err := s.recompute(ctx, sessionID); err != nil {
		s.log.Error("topology recompute failed", "session_id", sessionID, "error", err.Error())
	}
}

func (s *MeshTopologyService) recompute(ctx context.Context, sessionID string) error {
	started := time.Now()

	params, err := s.state.GetParams(ctx, sessionID)
	if err != nil {
		return err
	}

	positions, err := s.geo.AllPositions(ctx, sessionID)
	if err != nil {
		return err
	}

	// Only ships holding a live socket participate in routing; stale geo
	// entries (crashed clients) must not become phantom relays.
	online := make(map[string]bool)
	for _, id := range s.bcast.OnlineNodes(sessionID) {
		online[id] = true
	}
	liveNodes := positions[:0]
	for _, p := range positions {
		if online[p.ID] {
			liveNodes = append(liveNodes, p)
		}
	}

	edges, err := s.edges.List(ctx)
	if err != nil {
		return err
	}

	maxRange := domain.MaxRangeKm(params.TxPowerDbm)
	newTable := domain.BuildTopology(liveNodes, edges, maxRange)

	oldTable, err := s.routes.GetTable(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := s.routes.SaveTable(ctx, sessionID, newTable); err != nil {
		return err
	}

	changed := 0
	for nodeID, entry := range newTable {
		if routeUnchanged(oldTable, nodeID, entry) {
			continue
		}
		changed++
		s.bcast.SendToNode(sessionID, nodeID, protocol.RoutingUpdate{
			Event:              protocol.EvMeshRoutingUpdate,
			ParentTarget:       entry.Parent,
			ParentIsEdge:       entry.ParentIsEdge,
			DistanceToParentKm: round3(entry.DistanceKm),
			HopLevel:           entry.HopLevel,
			Status:             entry.Status,
		})
	}

	// Monitors receive the full table whenever anything changed (including
	// nodes that vanished from the table entirely).
	if changed > 0 || len(newTable) != len(oldTable) {
		s.bcast.BroadcastToMonitors(sessionID, s.tableEvent(newTable))
	}

	// Observability (PRD §3.5): topology calculation timing as JSON.
	s.log.Info("topology recomputed",
		"session_id", sessionID,
		"nodes", len(liveNodes),
		"edges", len(edges),
		"changed_routes", changed,
		"max_range_km", round3(maxRange),
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

func routeUnchanged(old map[string]domain.RouteEntry, nodeID string, entry domain.RouteEntry) bool {
	prev, ok := old[nodeID]
	if !ok {
		return false
	}
	return prev.Parent == entry.Parent &&
		prev.Status == entry.Status &&
		prev.HopLevel == entry.HopLevel &&
		math.Abs(prev.DistanceKm-entry.DistanceKm) <= distanceJitterKm
}

func (s *MeshTopologyService) tableEvent(table map[string]domain.RouteEntry) protocol.RoutingTable {
	routes := make([]domain.RouteEntry, 0, len(table))
	for _, e := range table {
		e.DistanceKm = round3(e.DistanceKm)
		routes = append(routes, e)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].NodeID < routes[j].NodeID })
	return protocol.RoutingTable{Event: protocol.EvMeshRoutingUpdate, Routes: routes}
}

// CurrentRoute returns a node's stored route for connect-time push.
func (s *MeshTopologyService) CurrentRoute(ctx context.Context, sessionID, nodeID string) (domain.RouteEntry, error) {
	return s.routes.GetRoute(ctx, sessionID, nodeID)
}

// TopologySnapshot assembles the dashboard's full picture: ship positions
// (with online flags), the routing table and every registered edge.
type TopologySnapshot struct {
	Nodes  []TopologyNode      `json:"nodes"`
	Routes []domain.RouteEntry `json:"routes"`
	Edges  []domain.Edge       `json:"edges"`
}

type TopologyNode struct {
	NodeID string  `json:"node_id"`
	Lat    float64 `json:"lat"`
	Lng    float64 `json:"lng"`
	Online bool    `json:"online"`
}

func (s *MeshTopologyService) Snapshot(ctx context.Context, sessionID string) (*TopologySnapshot, error) {
	positions, err := s.geo.AllPositions(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	table, err := s.routes.GetTable(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	edges, err := s.edges.List(ctx)
	if err != nil {
		return nil, err
	}

	online := make(map[string]bool)
	for _, id := range s.bcast.OnlineNodes(sessionID) {
		online[id] = true
	}

	snap := &TopologySnapshot{Edges: edges}
	for _, p := range positions {
		snap.Nodes = append(snap.Nodes, TopologyNode{
			NodeID: p.ID, Lat: p.Lat, Lng: p.Lng, Online: online[p.ID],
		})
	}
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].NodeID < snap.Nodes[j].NodeID })
	snap.Routes = s.tableEvent(table).Routes
	return snap, nil
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
