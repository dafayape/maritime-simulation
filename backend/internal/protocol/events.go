// Package protocol defines the WebSocket event contract shared by the
// gateway, the services, the nodesim harness and — by documentation — the
// Flutter mobile app and Next.js dashboard (SRS §4).
package protocol

import (
	"time"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// Client-to-server events (sent by mobile nodes).
const (
	EvNodePing     = "node:ping"
	EvNodeTransmit = "node:transmit"
	EvNodeAck      = "node:ack"
)

// Server-to-client events (received by mobile nodes).
const (
	EvMeshRoutingUpdate = "mesh:routing_update"
	EvMeshReceiveRF     = "mesh:receive_rf"
	// EvMeshAck relays a next-hop node:ack back to the transmitter. The SRS
	// names the client->server ACK but leaves the return leg unnamed; this
	// is the documented completion of that loop.
	EvMeshAck       = "mesh:ack"
	EvEnvSyncParams = "env:sync_params"
	// EvSchemaSync pushes the dynamic payload schema to nodes. The SRS
	// mandates the push (§3 endpoint 2) without naming the event; named here
	// consistently with env:sync_params.
	EvSchemaSync   = "schema:sync"
	EvSessionEnded = "session:ended"
	EvError        = "error"
)

// Monitor-only events (received by the Next.js dashboard observer socket).
const (
	EvPacketEvent = "packet:event"
	EvNodeStatus  = "node:status"
)

// Envelope sniffs the event discriminator of any inbound frame.
type Envelope struct {
	Event string `json:"event"`
}

// NodePing registers/refreshes a ship position (SRS §4A.1). It is also
// relayed verbatim to monitor sockets so the dashboard can move markers.
type NodePing struct {
	Event     string  `json:"event"`
	NodeID    string  `json:"node_id"`
	SessionID string  `json:"session_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
}

// NodeAck is the client acknowledgement for a received frame (SRS §4A.3).
type NodeAck struct {
	Event        string `json:"event"`
	PacketID     string `json:"packet_id"`
	ReceiverNode string `json:"receiver_node"`
	Status       string `json:"status"`
}

// RoutingUpdate is the per-node routing directive (SRS §4B.1): each ship
// only learns its own parent, replicating the limited memory of an ESP32.
type RoutingUpdate struct {
	Event              string  `json:"event"`
	ParentTarget       string  `json:"parent_target"`
	ParentIsEdge       bool    `json:"parent_is_edge"`
	DistanceToParentKm float64 `json:"distance_to_parent_km"`
	HopLevel           int     `json:"hop_level"`
	Status             string  `json:"status"`
}

// RoutingTable is the monitor-socket variant of mesh:routing_update carrying
// the whole DAG for dashboard polyline rendering.
type RoutingTable struct {
	Event  string              `json:"event"`
	Routes []domain.RouteEntry `json:"routes"`
}

// EnvSyncParams broadcasts the radio environment (SRS §4B.3, extended with
// the derived Data Link parameters so clients stay dumb).
type EnvSyncParams struct {
	Event           string  `json:"event"`
	SpreadingFactor int     `json:"sf"`
	MaxPayloadBytes int     `json:"max_payload_bytes"`
	AckTimeoutMs    int64   `json:"ack_timeout_ms"`
	MaxRetries      int     `json:"max_retries"`
	TxPowerDbm      int     `json:"tx_power_dbm"`
	MaxRangeKm      float64 `json:"max_range_km"`
	WeatherSeverity float64 `json:"weather_severity"`
}

// SchemaSync pushes the active dynamic payload schema.
type SchemaSync struct {
	Event                string               `json:"event"`
	Fields               []domain.SchemaField `json:"fields"`
	EstimatedPackedBytes int                  `json:"estimated_packed_bytes"`
}

// MeshAck closes the Data Link loop back to the transmitter. Duplicate=true
// means the backend re-acknowledged a retry whose original already got
// through (real LoRa receivers re-ACK duplicates instead of staying silent).
type MeshAck struct {
	Event        string `json:"event"`
	PacketID     string `json:"packet_id"`
	ReceiverNode string `json:"receiver_node"`
	Status       string `json:"status"`
	Duplicate    bool   `json:"duplicate,omitempty"`
}

// PacketEvent feeds the dashboard's live log panel and metric counters.
type PacketEvent struct {
	Event           string    `json:"event"`
	Type            string    `json:"type"`
	PacketID        string    `json:"packet_id"`
	FromNode        string    `json:"from_node"`
	ToNode          string    `json:"to_node"`
	OriginNode      string    `json:"origin_node,omitempty"`
	HopCount        int       `json:"hop_count,omitempty"`
	DistanceKm      float64   `json:"distance_km,omitempty"`
	LossProbability float64   `json:"loss_probability,omitempty"`
	Reason          string    `json:"reason,omitempty"`
	At              time.Time `json:"at"`
}

// PacketEvent types.
const (
	PktTransmit  = "transmit"
	PktForward   = "forward"
	PktDeliver   = "deliver"
	PktDrop      = "drop"
	PktDuplicate = "duplicate"
	PktAck       = "ack"
	PktAckDrop   = "ack_drop"
)

// NodeStatus tells monitors when ships join or leave the session.
type NodeStatus struct {
	Event  string `json:"event"`
	NodeID string `json:"node_id"`
	Online bool   `json:"online"`
}

// SessionEnded notifies every socket that the simulation was stopped.
type SessionEnded struct {
	Event     string `json:"event"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// ErrorEvent reports a rejected inbound frame to its sender.
type ErrorEvent struct {
	Event   string `json:"event"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
