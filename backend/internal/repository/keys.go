// Package repository is the data-access layer: Redis for hot simulation
// state (geospatial positions, routing table, dedup cache, counters) and
// PostgreSQL for configuration plus historical logs. Every Redis key lives
// here so the SRS §2B key contract is auditable in one place.
package repository

import "fmt"

func keyGeo(sessionID string) string { return fmt.Sprintf("sim:%s:geo", sessionID) }

func keyRoutes(sessionID string) string { return fmt.Sprintf("sim:%s:routes", sessionID) }

// keyRoutesFull stores the enriched routing entries (distance, hop level,
// status) used for diffing and the dashboard; sim:{id}:routes keeps the plain
// node->parent map exactly as specified in SRS §2B.3.
func keyRoutesFull(sessionID string) string { return fmt.Sprintf("sim:%s:routes_full", sessionID) }

// keyDedup includes the receiving node: the same packet_id legitimately
// traverses several hops (C->A then A->B), so deduplication must be scoped to
// one receiver. Documented deviation from the SRS §2B.2 literal key shape.
func keyDedup(sessionID, packetID, receiver string) string {
	return fmt.Sprintf("sim:%s:dedup:%s:%s", sessionID, packetID, receiver)
}

func keyParams(sessionID string) string { return fmt.Sprintf("sim:%s:params", sessionID) }

func keyStats(sessionID string) string { return fmt.Sprintf("sim:%s:stats", sessionID) }

func keyPingLimit(sessionID, nodeID string) string {
	return fmt.Sprintf("sim:%s:pinglimit:%s", sessionID, nodeID)
}

// keyAckSender remembers who transmitted a forwarded frame so the receiver's
// ACK can be relayed back through the virtual ether.
func keyAckSender(sessionID, packetID, receiver string) string {
	return fmt.Sprintf("sim:%s:acksender:%s:%s", sessionID, packetID, receiver)
}
