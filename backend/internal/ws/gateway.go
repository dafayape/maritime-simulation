package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/protocol"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/service"
)

// jobQueueSize bounds the inbound event queue shared by the worker pool. A
// broadcast storm beyond this backpressure point sheds frames (with a log)
// instead of exhausting memory.
const jobQueueSize = 8192

var nodeIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// Gateway upgrades HTTP requests to WebSocket connections and dispatches
// inbound events to the SimulationEngineService through a bounded worker
// pool (PRD: Worker Pools & Channels against broadcast storms).
type Gateway struct {
	hub    *Hub
	engine *service.SimulationEngineService
	log    *slog.Logger

	rootCtx  context.Context
	upgrader websocket.Upgrader

	jobs chan job
	wg   sync.WaitGroup
}

type job struct {
	sessionID string
	nodeID    string
	raw       []byte
}

// NewGateway wires the transport. rootCtx is the server's lifetime context:
// when it is cancelled every pump and worker drains and exits.
func NewGateway(rootCtx context.Context, hub *Hub, engine *service.SimulationEngineService, log *slog.Logger) *Gateway {
	return &Gateway{
		hub:     hub,
		engine:  engine,
		log:     log,
		rootCtx: rootCtx,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// The simulator is an open lab tool: browser dashboards and
			// Flutter clients connect from arbitrary origins.
			CheckOrigin: func(*http.Request) bool { return true },
		},
		jobs: make(chan job, jobQueueSize),
	}
}

// StartWorkers launches n event workers. Each worker recovers panics so a
// single poisoned frame can never take the pool down.
func (g *Gateway) StartWorkers(n int) {
	for i := 0; i < n; i++ {
		g.wg.Add(1)
		go func(workerID int) {
			defer g.wg.Done()
			for {
				select {
				case <-g.rootCtx.Done():
					return
				case j := <-g.jobs:
					g.safeProcess(workerID, j)
				}
			}
		}(i)
	}
	g.log.Info("websocket worker pool started", "workers", n, "queue", jobQueueSize)
}

// Wait blocks until every worker exited (called during graceful shutdown).
func (g *Gateway) Wait() {
	g.wg.Wait()
}

func (g *Gateway) safeProcess(workerID int, j job) {
	defer func() {
		if r := recover(); r != nil {
			g.log.Error("event worker panicked",
				"worker", workerID, "session_id", j.sessionID, "node_id", j.nodeID,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()

	var env protocol.Envelope
	if err := json.Unmarshal(j.raw, &env); err != nil {
		g.hub.SendToNode(j.sessionID, j.nodeID, protocol.ErrorEvent{
			Event: protocol.EvError, Code: "invalid_json", Message: err.Error(),
		})
		return
	}

	switch env.Event {
	case protocol.EvNodePing:
		var ping protocol.NodePing
		if err := json.Unmarshal(j.raw, &ping); err != nil {
			g.sendError(j, "invalid_ping", err.Error())
			return
		}
		// The connection identity is authoritative; payload ids are ignored
		// so one ship cannot move another.
		g.engine.HandlePing(g.rootCtx, j.sessionID, j.nodeID, ping.Lat, ping.Lng)

	case protocol.EvNodeTransmit:
		var pkt domain.TransmitPacket
		if err := json.Unmarshal(j.raw, &pkt); err != nil {
			g.sendError(j, "invalid_transmit", err.Error())
			return
		}
		g.engine.HandleTransmit(g.rootCtx, j.sessionID, j.nodeID, pkt)

	case protocol.EvNodeAck:
		var ack protocol.NodeAck
		if err := json.Unmarshal(j.raw, &ack); err != nil {
			g.sendError(j, "invalid_ack", err.Error())
			return
		}
		g.engine.HandleAck(g.rootCtx, j.sessionID, j.nodeID, ack)

	default:
		g.sendError(j, "unknown_event", "unsupported event: "+env.Event)
	}
}

func (g *Gateway) sendError(j job, code, msg string) {
	g.hub.SendToNode(j.sessionID, j.nodeID, protocol.ErrorEvent{
		Event: protocol.EvError, Code: code, Message: msg,
	})
}

// HandleNodeWS is GET /ws/nodes?session_id=...&node_id=... — the ship
// connection endpoint.
func (g *Gateway) HandleNodeWS(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	nodeID := r.URL.Query().Get("node_id")

	if sessionID == "" || !nodeIDRe.MatchString(nodeID) {
		http.Error(w, `{"status":"error","message":"session_id and a valid node_id query parameter are required"}`, http.StatusBadRequest)
		return
	}
	if !g.engine.SessionActive(r.Context(), sessionID) {
		http.Error(w, `{"status":"error","message":"unknown or inactive simulation session"}`, http.StatusNotFound)
		return
	}

	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.log.Warn("websocket upgrade failed", "error", err.Error())
		return
	}

	c := newClient(g.hub, conn, kindNode, sessionID, nodeID, g.log)
	g.hub.registerNode(c)

	go c.writePump()
	go c.readPump(g.rootCtx,
		func(raw []byte) {
			select {
			case g.jobs <- job{sessionID: sessionID, nodeID: nodeID, raw: raw}:
			default:
				g.log.Warn("event queue saturated, inbound frame shed",
					"session_id", sessionID, "node_id", nodeID)
			}
		},
		func() {
			if g.hub.unregisterNode(c) {
				g.engine.NodeDisconnected(g.rootCtx, sessionID, nodeID)
			}
		},
	)

	g.engine.NodeConnected(g.rootCtx, sessionID, nodeID)
}

// HandleMonitorWS is GET /ws/monitor?session_id=... — the read-only
// dashboard observer endpoint.
func (g *Gateway) HandleMonitorWS(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, `{"status":"error","message":"session_id query parameter is required"}`, http.StatusBadRequest)
		return
	}
	if !g.engine.SessionActive(r.Context(), sessionID) {
		http.Error(w, `{"status":"error","message":"unknown or inactive simulation session"}`, http.StatusNotFound)
		return
	}

	conn, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.log.Warn("websocket upgrade failed", "error", err.Error())
		return
	}

	c := newClient(g.hub, conn, kindMonitor, sessionID, "", g.log)
	g.hub.registerMonitor(c)

	go c.writePump()
	go c.readPump(g.rootCtx,
		func([]byte) {}, // monitors are read-only; inbound frames are ignored
		func() { g.hub.unregisterMonitor(c) },
	)

	// Prime the fresh dashboard with the current environment and topology.
	g.engine.MonitorConnected(g.rootCtx, sessionID, func(payload any) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		c.trySend(raw)
	})
	g.log.Info("monitor connected", "session_id", sessionID)
}
