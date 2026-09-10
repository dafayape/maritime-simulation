package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/protocol"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
)

// SimulationEngineService is the "Virtual Ether" (PRD §3.2): every frame a
// ship puts on the air passes through here. The pipeline mirrors a real
// LoRa MAC stack:
//
//  1. envelope + Base64 sanity (malformed radios don't transmit),
//  2. Spreading Factor byte ceiling — checked with len() on the decoded
//     byte slice, never on a string (Binary-First constraint),
//  3. TTL / max-hop guard,
//  4. target resolution (next ship or Virtual Edge),
//  5. probabilistic air-loss dice (distance × weather),
//  6. receiver-side deduplication (Redis SETNX — a surviving retry whose
//     original already arrived is re-ACKed, not re-delivered),
//  7. delivery: forward to the next ship's RX window, or unpack against the
//     dynamic schema and persist at the Edge, then relay the ACK back
//     through the same hostile air.
type SimulationEngineService struct {
	geo       *repository.GeoRepository
	state     *repository.StateRepository
	telemetry *repository.TelemetryRepository
	schemas   *DynamicSchemaService
	topology  *MeshTopologyService
	anomaly   *EnvironmentalAnomalyService
	bcast     Broadcaster
	log       *slog.Logger

	pingMinInterval time.Duration
	edges           *edgeCache
}

func NewSimulationEngineService(
	geo *repository.GeoRepository,
	state *repository.StateRepository,
	telemetry *repository.TelemetryRepository,
	edgeRepo *repository.EdgeRepository,
	schemas *DynamicSchemaService,
	topology *MeshTopologyService,
	anomaly *EnvironmentalAnomalyService,
	bcast Broadcaster,
	pingMinInterval time.Duration,
	log *slog.Logger,
) *SimulationEngineService {
	return &SimulationEngineService{
		geo:             geo,
		state:           state,
		telemetry:       telemetry,
		schemas:         schemas,
		topology:        topology,
		anomaly:         anomaly,
		bcast:           bcast,
		log:             log,
		pingMinInterval: pingMinInterval,
		edges:           newEdgeCache(edgeRepo, 30*time.Second),
	}
}

// SessionActive reports whether a session id has live cached parameters —
// the gateway's cheap admission check for new sockets.
func (s *SimulationEngineService) SessionActive(ctx context.Context, sessionID string) bool {
	_, err := s.state.GetParams(ctx, sessionID)
	return err == nil
}

// --- node:ping ----------------------------------------------------------------

// HandlePing refreshes a ship's position (rate-limited per PRD §3.5) and
// relays accepted pings to the dashboard so markers move in real time.
func (s *SimulationEngineService) HandlePing(ctx context.Context, sessionID, nodeID string, lat, lng float64) {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		s.bcast.SendToNode(sessionID, nodeID, protocol.ErrorEvent{
			Event: protocol.EvError, Code: "invalid_coordinates",
			Message: fmt.Sprintf("lat=%f lng=%f out of range", lat, lng),
		})
		return
	}

	allowed, err := s.state.AllowPing(ctx, sessionID, nodeID, s.pingMinInterval)
	if err != nil {
		s.log.Error("ping rate-limit check failed", "session_id", sessionID, "node_id", nodeID, "error", err.Error())
		return
	}
	if !allowed {
		_ = s.state.IncrStat(ctx, sessionID, StatPingsRateLimited)
		return
	}

	if err := s.geo.UpsertPosition(ctx, sessionID, nodeID, lat, lng); err != nil {
		s.log.Error("geo update failed", "session_id", sessionID, "node_id", nodeID, "error", err.Error())
		return
	}
	_ = s.state.IncrStat(ctx, sessionID, StatPingsAccepted)

	s.bcast.BroadcastToMonitors(sessionID, protocol.NodePing{
		Event: protocol.EvNodePing, NodeID: nodeID, SessionID: sessionID, Lat: lat, Lng: lng,
	})
}

// --- node:transmit --------------------------------------------------------------

// HandleTransmit pushes one frame through the virtual ether.
func (s *SimulationEngineService) HandleTransmit(ctx context.Context, sessionID, senderNodeID string, pkt domain.TransmitPacket) {
	if err := pkt.Validate(); err != nil {
		s.reject(ctx, sessionID, senderNodeID, pkt, "invalid_envelope", err.Error())
		return
	}
	transmitter := pkt.Transmitter()
	if transmitter != senderNodeID {
		s.reject(ctx, sessionID, senderNodeID, pkt, "transmitter_mismatch",
			fmt.Sprintf("routing_path ends with %q but frame came from %q", transmitter, senderNodeID))
		return
	}

	payload, err := base64.StdEncoding.DecodeString(pkt.BinaryPayloadB64)
	if err != nil {
		s.reject(ctx, sessionID, senderNodeID, pkt, "invalid_base64", err.Error())
		return
	}

	params, err := s.state.GetParams(ctx, sessionID)
	if err != nil {
		s.bcast.SendToNode(sessionID, senderNodeID, protocol.ErrorEvent{
			Event: protocol.EvError, Code: "session_unavailable",
			Message: "session parameters missing; was the session stopped?",
		})
		return
	}

	_ = s.state.IncrStat(ctx, sessionID, StatTransmitTotal)

	// LoRa physical constraint: the byte ceiling of the active SF.
	if limit := domain.MaxPayloadBytes(params.SpreadingFactor); len(payload) > limit {
		s.drop(ctx, sessionID, pkt, "sf_limit_exceeded", StatDroppedSFLimit, nil,
			fmt.Sprintf("payload %dB exceeds SF%d limit %dB", len(payload), params.SpreadingFactor, limit))
		return
	}

	// Mesh TTL constraint.
	if pkt.HopCount > domain.MaxHops {
		s.drop(ctx, sessionID, pkt, "max_hops_exceeded", StatDroppedMaxHops, nil,
			fmt.Sprintf("hop_count %d exceeds max %d", pkt.HopCount, domain.MaxHops))
		return
	}

	// Resolve the target: Virtual Edge or next ship.
	edge, isEdge, err := s.edges.get(ctx, pkt.TargetParent)
	if err != nil {
		s.log.Error("edge lookup failed", "session_id", sessionID, "error", err.Error())
		return
	}

	txPos, err := s.geo.Position(ctx, sessionID, transmitter)
	if errors.Is(err, domain.ErrNotFound) {
		s.drop(ctx, sessionID, pkt, "transmitter_no_position", StatDroppedNoPosition, nil,
			"transmitter has never pinged a GPS position")
		return
	}
	if err != nil {
		s.log.Error("geo read failed", "session_id", sessionID, "error", err.Error())
		return
	}

	var targetLat, targetLng float64
	if isEdge {
		targetLat, targetLng = edge.Latitude, edge.Longitude
	} else {
		if !s.bcast.NodeOnline(sessionID, pkt.TargetParent) {
			s.drop(ctx, sessionID, pkt, "target_offline", StatDroppedTargetOffline, nil,
				fmt.Sprintf("target %q holds no live connection", pkt.TargetParent))
			return
		}
		tPos, err := s.geo.Position(ctx, sessionID, pkt.TargetParent)
		if errors.Is(err, domain.ErrNotFound) {
			s.drop(ctx, sessionID, pkt, "target_no_position", StatDroppedNoPosition, nil,
				fmt.Sprintf("target %q has never pinged a GPS position", pkt.TargetParent))
			return
		}
		if err != nil {
			s.log.Error("geo read failed", "session_id", sessionID, "error", err.Error())
			return
		}
		targetLat, targetLng = tPos.Lat, tPos.Lng
	}

	// The dice: artificial packet loss over the air.
	dist := domain.HaversineKm(txPos.Lat, txPos.Lng, targetLat, targetLng)
	verdict := s.anomaly.Evaluate(dist, params)
	if verdict.Dropped {
		reason, stat := "air_loss", StatDroppedAirLoss
		if verdict.OutOfRange {
			reason, stat = "out_of_range", StatDroppedOutOfRange
		}
		s.drop(ctx, sessionID, pkt, reason, stat, &verdict, "")
		return
	}

	// The frame reached the receiver's radio: MAC-layer deduplication.
	firstTime, err := s.state.MarkPacketSeen(ctx, sessionID, pkt.PacketID, pkt.TargetParent)
	if err != nil {
		s.log.Error("dedup check failed", "session_id", sessionID, "packet_id", pkt.PacketID, "error", err.Error())
		return
	}
	if !firstTime {
		s.handleDuplicate(ctx, sessionID, pkt, transmitter, dist, params, verdict)
		return
	}

	if isEdge {
		s.deliverToEdge(ctx, sessionID, pkt, payload, edge, transmitter, dist, params, verdict)
		return
	}
	s.forwardToNode(ctx, sessionID, pkt, transmitter, dist, verdict)
}

// handleDuplicate filters a retry whose original already arrived (SRS §2B.2)
// and replays the ACK — a silent receiver would strand the sender in a retry
// loop for a packet that actually got through.
func (s *SimulationEngineService) handleDuplicate(
	ctx context.Context, sessionID string, pkt domain.TransmitPacket,
	transmitter string, dist float64, params repository.SessionParams,
	verdict TraversalVerdict,
) {
	_ = s.state.IncrStat(ctx, sessionID, StatDuplicatesFiltered)
	s.emitPacketEvent(sessionID, protocol.PacketEvent{
		Type: protocol.PktDuplicate, PacketID: pkt.PacketID,
		FromNode: transmitter, ToNode: pkt.TargetParent, OriginNode: pkt.OriginNode,
		HopCount: pkt.HopCount, DistanceKm: verdict.DistanceKm, LossProbability: verdict.LossPercent,
		Reason: "duplicate retry filtered",
	})
	s.log.Debug("duplicate packet filtered",
		"session_id", sessionID, "packet_id", pkt.PacketID, "receiver", pkt.TargetParent)

	back := s.anomaly.Evaluate(dist, params)
	if back.Dropped {
		_ = s.state.IncrStat(ctx, sessionID, StatAcksDropped)
		return
	}
	s.bcast.SendToNode(sessionID, transmitter, protocol.MeshAck{
		Event: protocol.EvMeshAck, PacketID: pkt.PacketID,
		ReceiverNode: pkt.TargetParent, Status: "received", Duplicate: true,
	})
}

// forwardToNode simulates the RF propagation into the next ship's RX window.
func (s *SimulationEngineService) forwardToNode(
	ctx context.Context, sessionID string, pkt domain.TransmitPacket,
	transmitter string, dist float64, verdict TraversalVerdict,
) {
	if err := s.state.RememberAckSender(ctx, sessionID, pkt.PacketID, pkt.TargetParent, transmitter); err != nil {
		s.log.Error("ack-sender bookkeeping failed", "session_id", sessionID, "error", err.Error())
	}

	rf := pkt
	rf.Event = protocol.EvMeshReceiveRF
	s.bcast.SendToNode(sessionID, pkt.TargetParent, rf)

	_ = s.state.IncrStat(ctx, sessionID, StatForwardedToNode)
	s.emitPacketEvent(sessionID, protocol.PacketEvent{
		Type: protocol.PktForward, PacketID: pkt.PacketID,
		FromNode: transmitter, ToNode: pkt.TargetParent, OriginNode: pkt.OriginNode,
		HopCount: pkt.HopCount, DistanceKm: verdict.DistanceKm, LossProbability: verdict.LossPercent,
	})
	s.log.Debug("packet forwarded",
		"session_id", sessionID, "packet_id", pkt.PacketID,
		"from", transmitter, "to", pkt.TargetParent, "hop", pkt.HopCount)
}

// deliverToEdge is the finish line: unpack the binary payload against the
// dynamic schema, persist the telemetry row, and ACK back through the air.
func (s *SimulationEngineService) deliverToEdge(
	ctx context.Context, sessionID string, pkt domain.TransmitPacket, payload []byte,
	edge domain.Edge, transmitter string, dist float64, params repository.SessionParams,
	verdict TraversalVerdict,
) {
	var decoded map[string]any
	schema, err := s.schemas.GetActive(ctx, sessionID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// No schema injected yet: keep the raw bytes retrievable.
		decoded = map[string]any{"_raw_b64": pkt.BinaryPayloadB64}
	case err != nil:
		s.log.Error("schema lookup failed", "session_id", sessionID, "error", err.Error())
		decoded = map[string]any{"_raw_b64": pkt.BinaryPayloadB64}
	default:
		var problems []string
		decoded, problems = domain.DecodePayload(payload, schema.Fields)
		if decoded == nil {
			decoded = map[string]any{"_raw_b64": pkt.BinaryPayloadB64}
		}
		if len(problems) > 0 {
			decoded["_schema_problems"] = problems
		}
	}

	edgeID := edge.ID
	row := &domain.TelemetryLog{
		SessionID:    sessionID,
		OriginNodeID: pkt.OriginNode,
		EdgeID:       &edgeID,
		// Snapshot the code now, at the moment of delivery — Insert freezes
		// it into edge_code_snapshot so a later edge deletion (ON DELETE SET
		// NULL on edge_id) never erases this row's "Edge" column.
		EdgeCode:       edge.EdgeCode,
		HopCount:       pkt.HopCount,
		RoutingPath:    pkt.PathString(edge.EdgeCode),
		DecodedPayload: decoded,
	}
	if err := s.telemetry.Insert(ctx, row); err != nil {
		// Radio-layer success is independent of our persistence problem:
		// still ACK, loudly log the storage fault.
		s.log.Error("telemetry insert failed", "session_id", sessionID, "packet_id", pkt.PacketID, "error", err.Error())
	}

	_ = s.state.IncrStat(ctx, sessionID, StatDeliveredToEdge)
	s.emitPacketEvent(sessionID, protocol.PacketEvent{
		Type: protocol.PktDeliver, PacketID: pkt.PacketID,
		FromNode: transmitter, ToNode: edge.EdgeCode, OriginNode: pkt.OriginNode,
		HopCount: pkt.HopCount, DistanceKm: verdict.DistanceKm, LossProbability: verdict.LossPercent,
	})
	s.log.Info("packet delivered to edge",
		"session_id", sessionID, "packet_id", pkt.PacketID, "origin", pkt.OriginNode,
		"edge", edge.EdgeCode, "hops", pkt.HopCount, "path", row.RoutingPath)

	// The Edge acknowledges; the ACK rides the same hostile air back.
	back := s.anomaly.Evaluate(dist, params)
	if back.Dropped {
		_ = s.state.IncrStat(ctx, sessionID, StatAcksDropped)
		s.emitPacketEvent(sessionID, protocol.PacketEvent{
			Type: protocol.PktAckDrop, PacketID: pkt.PacketID,
			FromNode: edge.EdgeCode, ToNode: transmitter,
			DistanceKm: back.DistanceKm, LossProbability: back.LossPercent,
			Reason: "ack lost in the air",
		})
		return
	}
	s.bcast.SendToNode(sessionID, transmitter, protocol.MeshAck{
		Event: protocol.EvMeshAck, PacketID: pkt.PacketID,
		ReceiverNode: edge.EdgeCode, Status: "received",
	})
	_ = s.state.IncrStat(ctx, sessionID, StatAcksRelayed)
	s.emitPacketEvent(sessionID, protocol.PacketEvent{
		Type: protocol.PktAck, PacketID: pkt.PacketID,
		FromNode: edge.EdgeCode, ToNode: transmitter,
	})
}

// --- node:ack -------------------------------------------------------------------

// HandleAck relays a receiver's acknowledgement back to whoever transmitted
// the frame; the ACK is subject to the same loss probability (SRS §5B.4).
func (s *SimulationEngineService) HandleAck(ctx context.Context, sessionID, receiverNodeID string, ack protocol.NodeAck) {
	if ack.PacketID == "" {
		return
	}
	sender, err := s.state.TakeAckSender(ctx, sessionID, ack.PacketID, receiverNodeID)
	if errors.Is(err, domain.ErrNotFound) {
		_ = s.state.IncrStat(ctx, sessionID, StatAcksStale)
		s.log.Debug("stale ack ignored", "session_id", sessionID, "packet_id", ack.PacketID, "receiver", receiverNodeID)
		return
	}
	if err != nil {
		s.log.Error("ack-sender lookup failed", "session_id", sessionID, "error", err.Error())
		return
	}

	params, err := s.state.GetParams(ctx, sessionID)
	if err != nil {
		return
	}

	rxPos, rxErr := s.geo.Position(ctx, sessionID, receiverNodeID)
	txPos, txErr := s.geo.Position(ctx, sessionID, sender)
	if rxErr != nil || txErr != nil {
		_ = s.state.IncrStat(ctx, sessionID, StatAcksDropped)
		return
	}

	dist := domain.HaversineKm(rxPos.Lat, rxPos.Lng, txPos.Lat, txPos.Lng)
	verdict := s.anomaly.Evaluate(dist, params)
	if verdict.Dropped {
		_ = s.state.IncrStat(ctx, sessionID, StatAcksDropped)
		s.emitPacketEvent(sessionID, protocol.PacketEvent{
			Type: protocol.PktAckDrop, PacketID: ack.PacketID,
			FromNode: receiverNodeID, ToNode: sender,
			DistanceKm: verdict.DistanceKm, LossProbability: verdict.LossPercent,
			Reason: "ack lost in the air",
		})
		s.log.Debug("ack dropped in the air",
			"session_id", sessionID, "packet_id", ack.PacketID,
			"loss_pct", verdict.LossPercent, "distance_km", verdict.DistanceKm)
		return
	}

	status := ack.Status
	if status == "" {
		status = "received"
	}
	s.bcast.SendToNode(sessionID, sender, protocol.MeshAck{
		Event: protocol.EvMeshAck, PacketID: ack.PacketID,
		ReceiverNode: receiverNodeID, Status: status,
	})
	_ = s.state.IncrStat(ctx, sessionID, StatAcksRelayed)
	s.emitPacketEvent(sessionID, protocol.PacketEvent{
		Type: protocol.PktAck, PacketID: ack.PacketID,
		FromNode: receiverNodeID, ToNode: sender,
	})
}

// --- connection lifecycle --------------------------------------------------------

// NodeConnected pushes the current environment, schema and route to a ship
// that just joined, so late joiners are immediately operational.
func (s *SimulationEngineService) NodeConnected(ctx context.Context, sessionID, nodeID string) {
	s.bcast.BroadcastToMonitors(sessionID, protocol.NodeStatus{
		Event: protocol.EvNodeStatus, NodeID: nodeID, Online: true,
	})

	if params, err := s.state.GetParams(ctx, sessionID); err == nil {
		s.bcast.SendToNode(sessionID, nodeID, EnvParamsFrom(params))
	}
	if schema, err := s.schemas.GetActive(ctx, sessionID); err == nil {
		s.bcast.SendToNode(sessionID, nodeID, protocol.SchemaSync{
			Event:                protocol.EvSchemaSync,
			Fields:               schema.Fields,
			EstimatedPackedBytes: domain.EstimatePackedBytes(schema.Fields),
		})
	}
	if route, err := s.topology.CurrentRoute(ctx, sessionID, nodeID); err == nil {
		s.bcast.SendToNode(sessionID, nodeID, protocol.RoutingUpdate{
			Event:              protocol.EvMeshRoutingUpdate,
			ParentTarget:       route.Parent,
			ParentIsEdge:       route.ParentIsEdge,
			DistanceToParentKm: route.DistanceKm,
			HopLevel:           route.HopLevel,
			Status:             route.Status,
		})
	}
	s.log.Info("node connected", "session_id", sessionID, "node_id", nodeID)
}

// NodeDisconnected removes the ship from the spatial index so the next
// topology tick re-routes its children.
func (s *SimulationEngineService) NodeDisconnected(ctx context.Context, sessionID, nodeID string) {
	if err := s.geo.RemoveNode(ctx, sessionID, nodeID); err != nil {
		s.log.Warn("geo remove failed", "session_id", sessionID, "node_id", nodeID, "error", err.Error())
	}
	s.bcast.BroadcastToMonitors(sessionID, protocol.NodeStatus{
		Event: protocol.EvNodeStatus, NodeID: nodeID, Online: false,
	})
	s.log.Info("node disconnected", "session_id", sessionID, "node_id", nodeID)
}

// MonitorConnected sends a fresh dashboard the current routing table so it
// can render the map before the next topology tick.
func (s *SimulationEngineService) MonitorConnected(ctx context.Context, sessionID string, send func(payload any)) {
	if params, err := s.state.GetParams(ctx, sessionID); err == nil {
		send(EnvParamsFrom(params))
	}
	if snap, err := s.topology.Snapshot(ctx, sessionID); err == nil {
		send(protocol.RoutingTable{Event: protocol.EvMeshRoutingUpdate, Routes: snap.Routes})
	}
}

// --- helpers ----------------------------------------------------------------------

// reject reports a protocol violation back to the offending sender.
func (s *SimulationEngineService) reject(ctx context.Context, sessionID, senderNodeID string, pkt domain.TransmitPacket, code, msg string) {
	_ = s.state.IncrStat(ctx, sessionID, StatDroppedInvalid)
	s.bcast.SendToNode(sessionID, senderNodeID, protocol.ErrorEvent{
		Event: protocol.EvError, Code: code, Message: msg,
	})
	s.log.Warn("transmit rejected",
		"session_id", sessionID, "node_id", senderNodeID,
		"packet_id", pkt.PacketID, "code", code, "detail", msg)
}

// drop silences a frame mid-air: the sender learns about it only through its
// own ACK timeout, exactly like real radio.
func (s *SimulationEngineService) drop(
	ctx context.Context, sessionID string, pkt domain.TransmitPacket,
	reason, statField string, verdict *TraversalVerdict, detail string,
) {
	_ = s.state.IncrStat(ctx, sessionID, statField)

	ev := protocol.PacketEvent{
		Type: protocol.PktDrop, PacketID: pkt.PacketID,
		FromNode: pkt.Transmitter(), ToNode: pkt.TargetParent, OriginNode: pkt.OriginNode,
		HopCount: pkt.HopCount, Reason: reason,
	}
	logAttrs := []any{
		"session_id", sessionID, "packet_id", pkt.PacketID,
		"from", pkt.Transmitter(), "to", pkt.TargetParent, "reason", reason,
	}
	if verdict != nil {
		ev.DistanceKm = verdict.DistanceKm
		ev.LossProbability = verdict.LossPercent
		logAttrs = append(logAttrs, "distance_km", verdict.DistanceKm, "loss_pct", verdict.LossPercent)
	}
	if detail != "" {
		logAttrs = append(logAttrs, "detail", detail)
	}
	s.emitPacketEvent(sessionID, ev)

	// Air loss is business-as-usual noise (debug); rule violations are
	// actionable (info).
	if reason == "air_loss" || reason == "out_of_range" {
		s.log.Debug("packet dropped", logAttrs...)
	} else {
		s.log.Info("packet dropped", logAttrs...)
	}
}

func (s *SimulationEngineService) emitPacketEvent(sessionID string, ev protocol.PacketEvent) {
	ev.Event = protocol.EvPacketEvent
	ev.At = time.Now().UTC()
	s.bcast.BroadcastToMonitors(sessionID, ev)
}

// --- edge cache --------------------------------------------------------------------

// edgeCache keeps the (tiny) virtual_edges table in memory with a short TTL
// so the per-packet hot path never queries PostgreSQL.
type edgeCache struct {
	repo *repository.EdgeRepository
	ttl  time.Duration

	mu       sync.RWMutex
	byCode   map[string]domain.Edge
	loadedAt time.Time
}

func newEdgeCache(repo *repository.EdgeRepository, ttl time.Duration) *edgeCache {
	return &edgeCache{repo: repo, ttl: ttl}
}

func (c *edgeCache) get(ctx context.Context, code string) (domain.Edge, bool, error) {
	c.mu.RLock()
	fresh := c.byCode != nil && time.Since(c.loadedAt) < c.ttl
	e, ok := c.byCode[code]
	c.mu.RUnlock()
	if fresh {
		return e, ok, nil
	}

	edges, err := c.repo.List(ctx)
	if err != nil {
		if c.byCode != nil {
			// Serve stale data during a transient DB hiccup.
			return e, ok, nil
		}
		return domain.Edge{}, false, err
	}

	byCode := make(map[string]domain.Edge, len(edges))
	for _, edge := range edges {
		byCode[edge.EdgeCode] = edge
	}

	c.mu.Lock()
	c.byCode = byCode
	c.loadedAt = time.Now()
	e, ok = byCode[code]
	c.mu.Unlock()
	return e, ok, nil
}
