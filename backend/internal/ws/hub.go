// Package ws is the WebSocket transport layer: it upgrades connections,
// keeps the per-session client registries, and pumps frames between the
// network and the SimulationEngineService worker pool.
package ws

import (
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
)

// Hub tracks every live socket, split by session and by kind (ship node vs
// dashboard monitor). It implements service.Broadcaster.
type Hub struct {
	log        *slog.Logger
	sendBuffer int

	mu       sync.RWMutex
	nodes    map[string]map[string]*Client // sessionID -> nodeID -> client
	monitors map[string]map[*Client]bool   // sessionID -> observer set
}

func NewHub(sendBuffer int, log *slog.Logger) *Hub {
	if sendBuffer <= 0 {
		sendBuffer = 256
	}
	return &Hub{
		log:        log,
		sendBuffer: sendBuffer,
		nodes:      make(map[string]map[string]*Client),
		monitors:   make(map[string]map[*Client]bool),
	}
}

// --- service.Broadcaster implementation --------------------------------------

func (h *Hub) SendToNode(sessionID, nodeID string, payload any) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("marshal outbound payload failed", "error", err.Error())
		return false
	}
	h.mu.RLock()
	c := h.nodes[sessionID][nodeID]
	h.mu.RUnlock()
	if c == nil {
		return false
	}
	return c.trySend(raw)
}

func (h *Hub) BroadcastToNodes(sessionID string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("marshal broadcast payload failed", "error", err.Error())
		return
	}
	for _, c := range h.snapshotNodes(sessionID) {
		c.trySend(raw)
	}
}

func (h *Hub) BroadcastToMonitors(sessionID string, payload any) {
	h.mu.RLock()
	empty := len(h.monitors[sessionID]) == 0
	h.mu.RUnlock()
	if empty {
		return
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("marshal monitor payload failed", "error", err.Error())
		return
	}
	for _, c := range h.snapshotMonitors(sessionID) {
		c.trySend(raw)
	}
}

func (h *Hub) NodeOnline(sessionID, nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.nodes[sessionID][nodeID] != nil
}

func (h *Hub) OnlineNodes(sessionID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.nodes[sessionID]))
	for id := range h.nodes[sessionID] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// KickSession force-closes every socket of a session (session stop).
func (h *Hub) KickSession(sessionID string) {
	var clients []*Client
	clients = append(clients, h.snapshotNodes(sessionID)...)
	clients = append(clients, h.snapshotMonitors(sessionID)...)
	for _, c := range clients {
		c.Close()
	}
}

// --- registry ------------------------------------------------------------------

// registerNode adds a ship connection. A reconnect with the same node_id
// replaces (and closes) the previous socket.
func (h *Hub) registerNode(c *Client) {
	h.mu.Lock()
	if h.nodes[c.sessionID] == nil {
		h.nodes[c.sessionID] = make(map[string]*Client)
	}
	old := h.nodes[c.sessionID][c.nodeID]
	h.nodes[c.sessionID][c.nodeID] = c
	h.mu.Unlock()

	if old != nil {
		h.log.Warn("node reconnected, closing previous socket",
			"session_id", c.sessionID, "node_id", c.nodeID)
		old.Close()
	}
}

// unregisterNode removes a ship connection, but only if the registry still
// points at this exact client (a reconnect may already have replaced it).
func (h *Hub) unregisterNode(c *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.nodes[c.sessionID][c.nodeID] != c {
		return false
	}
	delete(h.nodes[c.sessionID], c.nodeID)
	if len(h.nodes[c.sessionID]) == 0 {
		delete(h.nodes, c.sessionID)
	}
	return true
}

func (h *Hub) registerMonitor(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.monitors[c.sessionID] == nil {
		h.monitors[c.sessionID] = make(map[*Client]bool)
	}
	h.monitors[c.sessionID][c] = true
}

func (h *Hub) unregisterMonitor(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.monitors[c.sessionID], c)
	if len(h.monitors[c.sessionID]) == 0 {
		delete(h.monitors, c.sessionID)
	}
}

func (h *Hub) snapshotNodes(sessionID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*Client, 0, len(h.nodes[sessionID]))
	for _, c := range h.nodes[sessionID] {
		clients = append(clients, c)
	}
	return clients
}

func (h *Hub) snapshotMonitors(sessionID string) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*Client, 0, len(h.monitors[sessionID]))
	for c := range h.monitors[sessionID] {
		clients = append(clients, c)
	}
	return clients
}

// Shutdown closes every socket across all sessions (graceful stop).
func (h *Hub) Shutdown() {
	h.mu.RLock()
	var clients []*Client
	for _, m := range h.nodes {
		for _, c := range m {
			clients = append(clients, c)
		}
	}
	for _, m := range h.monitors {
		for c := range m {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.Close()
	}
	h.log.Info("websocket hub shut down", "closed_connections", len(clients))
}

// Counts reports live connection totals for health endpoints.
func (h *Hub) Counts() (nodeConns, monitorConns int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, m := range h.nodes {
		nodeConns += len(m)
	}
	for _, m := range h.monitors {
		monitorConns += len(m)
	}
	return nodeConns, monitorConns
}
