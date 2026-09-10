package domain

import (
	"fmt"
	"strings"
)

// TransmitPacket is the mesh frame envelope (SRS §4A.2). The binary payload
// travels as Base64 inside the JSON envelope and is only ever handled as a
// byte slice after decoding — never as a string (Binary-First constraint).
type TransmitPacket struct {
	Event            string   `json:"event"`
	PacketID         string   `json:"packet_id"`
	OriginNode       string   `json:"origin_node"`
	TargetParent     string   `json:"target_parent"`
	HopCount         int      `json:"hop_count"`
	RoutingPath      []string `json:"routing_path"`
	BinaryPayloadB64 string   `json:"binary_payload_b64"`
}

const maxIDLength = 100

// Validate enforces envelope sanity before the packet enters the virtual
// ether. It does NOT check the SF byte limit — that requires the decoded
// byte slice and the session state, which is the engine's job.
func (p *TransmitPacket) Validate() error {
	switch {
	case p.PacketID == "" || len(p.PacketID) > maxIDLength:
		return fmt.Errorf("%w: packet_id is required (max %d chars)", ErrValidation, maxIDLength)
	case p.OriginNode == "" || len(p.OriginNode) > maxIDLength:
		return fmt.Errorf("%w: origin_node is required (max %d chars)", ErrValidation, maxIDLength)
	case p.TargetParent == "" || len(p.TargetParent) > maxIDLength:
		return fmt.Errorf("%w: target_parent is required (max %d chars)", ErrValidation, maxIDLength)
	case p.HopCount < 1:
		return fmt.Errorf("%w: hop_count must be >= 1", ErrValidation)
	case len(p.RoutingPath) == 0:
		return fmt.Errorf("%w: routing_path must not be empty", ErrValidation)
	case len(p.RoutingPath) > MaxHops+1:
		return fmt.Errorf("%w: routing_path exceeds max hops", ErrValidation)
	case p.BinaryPayloadB64 == "":
		return fmt.Errorf("%w: binary_payload_b64 is required", ErrValidation)
	}
	for _, hop := range p.RoutingPath {
		if hop == "" || len(hop) > maxIDLength {
			return fmt.Errorf("%w: routing_path contains an invalid node id", ErrValidation)
		}
	}
	return nil
}

// Transmitter returns the node currently putting this frame on the air: the
// last entry of the routing path (the origin on hop 1, otherwise the relay).
func (p *TransmitPacket) Transmitter() string {
	return p.RoutingPath[len(p.RoutingPath)-1]
}

// PathString renders "KPL-001->KPL-002->EDGE-1" for telemetry_logs.
func (p *TransmitPacket) PathString(edgeCode string) string {
	return strings.Join(append(append([]string{}, p.RoutingPath...), edgeCode), "->")
}
