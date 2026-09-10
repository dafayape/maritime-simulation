// Package service is the application layer: it orchestrates the simulation
// without owning transport or storage details (PRD §3.2).
package service

// Broadcaster is the narrow view of the WebSocket hub that services need.
// The ws package implements it; defining the interface here keeps the
// dependency arrow pointing transport -> application, never the reverse.
type Broadcaster interface {
	// SendToNode delivers one message to a single ship. It returns false
	// when the node is offline or its send buffer is saturated.
	SendToNode(sessionID, nodeID string, payload any) bool
	// BroadcastToNodes fans a message out to every ship in a session.
	BroadcastToNodes(sessionID string, payload any)
	// BroadcastToMonitors fans a message out to every dashboard observer.
	BroadcastToMonitors(sessionID string, payload any)
	// NodeOnline reports whether a ship currently holds a live connection.
	NodeOnline(sessionID, nodeID string) bool
	// OnlineNodes lists the ships currently connected to a session.
	OnlineNodes(sessionID string) []string
	// KickSession force-closes every socket of a stopped session.
	KickSession(sessionID string)
}
