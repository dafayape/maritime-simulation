// Command nodesim spawns a fleet of virtual LoRa ship nodes against the
// simulator backend. It is the backend's own acceptance harness — and, until
// the Flutter app ships, the reference implementation of the WebSocket
// protocol: GPS pings, schema-driven MessagePack payloads, ACK waiting with
// SF-dependent timeouts, auto-retry (max 3), store-and-forward relaying and
// client-side duplicate suppression.
//
// Example (multi-hop topology with a low TX power):
//
//	go run ./cmd/nodesim -api http://localhost:8080 -nodes 50 -tx 10 \
//	    -spread-km 8 -duration 60s
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vmihailenco/msgpack/v5"
)

type options struct {
	api           string
	session       string
	sessionName   string
	sf            int
	tx            int
	weather       float64
	injectSchema  bool
	nodes         int
	centerLat     float64
	centerLng     float64
	spreadKm      float64
	speedKmh      float64
	pingInterval  time.Duration
	transmitEvery time.Duration
	duration      time.Duration
	verbose       bool
	ignoreSF      bool
}

type fleetStats struct {
	sent      atomic.Int64
	acked     atomic.Int64
	retries   atomic.Int64
	failed    atomic.Int64 // link declared dead after max retries
	forwarded atomic.Int64
	received  atomic.Int64
	dupSeen   atomic.Int64
}

func main() {
	opt := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sessionID, err := ensureSession(ctx, opt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "session setup failed:", err)
		os.Exit(1)
	}
	fmt.Printf("session: %s\n", sessionID)

	stats := &fleetStats{}
	var wg sync.WaitGroup
	runCtx, cancel := context.WithTimeout(ctx, opt.duration)
	defer cancel()

	for i := 1; i <= opt.nodes; i++ {
		nodeID := fmt.Sprintf("KPL-%03d", i)
		lat, lng := scatter(opt.centerLat, opt.centerLng, opt.spreadKm)
		n := &node{
			id: nodeID, sessionID: sessionID, opt: opt, stats: stats,
			lat: lat, lng: lng,
			heading:    rand.Float64() * 2 * math.Pi,
			ackTimeout: 2 * time.Second,
			maxPayload: 242,
			seen:       make(map[string]bool),
			waiters:    make(map[string]chan struct{}),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			n.run(runCtx)
		}()
		// Stagger connections so 50 sockets don't slam in one instant.
		time.Sleep(20 * time.Millisecond)
	}

	wg.Wait()
	printSummary(opt, sessionID, stats)
}

func parseFlags() options {
	var opt options
	flag.StringVar(&opt.api, "api", "http://localhost:8080", "backend base URL")
	flag.StringVar(&opt.session, "session", "", "existing session id (created when empty)")
	flag.StringVar(&opt.sessionName, "name", "nodesim fleet run", "session name when creating")
	flag.IntVar(&opt.sf, "sf", 7, "spreading factor when creating the session")
	flag.IntVar(&opt.tx, "tx", 10, "TX power dBm when creating (10 dBm ≈ 3.1 km range → multi-hop)")
	flag.Float64Var(&opt.weather, "weather", 1.0, "weather severity when creating (0..2)")
	flag.BoolVar(&opt.injectSchema, "schema", true, "inject the default fishing schema when creating")
	flag.IntVar(&opt.nodes, "nodes", 10, "number of virtual ships")
	center := flag.String("center", "-6.9875,106.5504", "fleet center lat,lng (default: seed edge)")
	flag.Float64Var(&opt.spreadKm, "spread-km", 8, "initial scatter radius in km")
	flag.Float64Var(&opt.speedKmh, "speed-kmh", 10, "ship random-walk speed")
	flag.DurationVar(&opt.pingInterval, "interval", 3*time.Second, "GPS ping interval")
	flag.DurationVar(&opt.transmitEvery, "transmit-every", 10*time.Second, "telemetry transmit interval per ship")
	flag.DurationVar(&opt.duration, "duration", 60*time.Second, "how long the fleet sails")
	flag.BoolVar(&opt.verbose, "v", false, "verbose per-packet logging")
	flag.BoolVar(&opt.ignoreSF, "ignore-sf", false,
		"misbehaving-client mode: transmit even when the payload exceeds the SF window (proves the server-side drop)")
	flag.Parse()

	parts := strings.Split(*center, ",")
	if len(parts) == 2 {
		fmt.Sscanf(parts[0], "%f", &opt.centerLat)
		fmt.Sscanf(parts[1], "%f", &opt.centerLng)
	}
	return opt
}

// --- REST bootstrap ---------------------------------------------------------------

func ensureSession(ctx context.Context, opt options) (string, error) {
	if opt.session != "" {
		return opt.session, nil
	}
	resp, err := postJSON(ctx, opt.api+"/api/v1/simulations", map[string]any{
		"session_name":     opt.sessionName,
		"spreading_factor": opt.sf,
		"tx_power_dbm":     opt.tx,
		"weather_severity": opt.weather,
	})
	if err != nil {
		return "", err
	}
	data, _ := resp["data"].(map[string]any)
	sessionID, _ := data["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("no session_id in response: %v", resp)
	}

	if opt.injectSchema {
		_, err = postJSON(ctx, fmt.Sprintf("%s/api/v1/simulations/%s/schema", opt.api, sessionID), map[string]any{
			"fields": []map[string]string{
				{"name": "lat", "type": "float32"},
				{"name": "lng", "type": "float32"},
				{"name": "berat_kg", "type": "uint16"},
				{"name": "jenis_ikan", "type": "string_10"},
				{"name": "fresh", "type": "bool"},
			},
		})
		if err != nil {
			return "", fmt.Errorf("schema injection: %w", err)
		}
	}
	return sessionID, nil
}

func postJSON(ctx context.Context, endpoint string, body any) (map[string]any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s -> %d: %v", endpoint, resp.StatusCode, parsed["message"])
	}
	return parsed, nil
}

// --- the virtual ship ---------------------------------------------------------------

type schemaField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type node struct {
	id        string
	sessionID string
	opt       options
	stats     *fleetStats

	conn    *websocket.Conn
	writeMu sync.Mutex

	lat, lng float64
	heading  float64

	mu           sync.Mutex
	parent       string
	parentIsEdge bool
	routed       bool
	ackTimeout   time.Duration
	maxPayload   int
	fields       []schemaField
	seen         map[string]bool
	waiters      map[string]chan struct{}
	pktSeq       int
}

func (n *node) run(ctx context.Context) {
	wsURL, err := nodeWSURL(n.opt.api, n.sessionID, n.id)
	if err != nil {
		fmt.Fprintln(os.Stderr, n.id, "bad ws url:", err)
		return
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, n.id, "connect failed:", err)
		return
	}
	n.conn = conn
	defer conn.Close()

	readerDone := make(chan struct{})
	go n.reader(ctx, readerDone)

	n.sendPing() // register a position immediately

	pingT := time.NewTicker(n.opt.pingInterval)
	defer pingT.Stop()
	// Jitter transmit phase so the fleet doesn't fire in lockstep.
	txT := time.NewTicker(n.opt.transmitEvery + time.Duration(rand.Int64N(int64(n.opt.transmitEvery/2))))
	defer txT.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = n.writeControlClose()
			return
		case <-readerDone:
			return
		case <-pingT.C:
			n.move()
			n.sendPing()
		case <-txT.C:
			go n.transmitTelemetry(ctx)
		}
	}
}

func (n *node) reader(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	for {
		_, raw, err := n.conn.ReadMessage()
		if err != nil {
			return
		}
		var env struct {
			Event string `json:"event"`
		}
		if json.Unmarshal(raw, &env) != nil {
			continue
		}

		switch env.Event {
		case "env:sync_params":
			var p struct {
				MaxPayloadBytes int   `json:"max_payload_bytes"`
				AckTimeoutMs    int64 `json:"ack_timeout_ms"`
			}
			if json.Unmarshal(raw, &p) == nil && p.AckTimeoutMs > 0 {
				n.mu.Lock()
				n.ackTimeout = time.Duration(p.AckTimeoutMs) * time.Millisecond
				n.maxPayload = p.MaxPayloadBytes
				n.mu.Unlock()
			}

		case "schema:sync":
			var s struct {
				Fields []schemaField `json:"fields"`
			}
			if json.Unmarshal(raw, &s) == nil {
				n.mu.Lock()
				n.fields = s.Fields
				n.mu.Unlock()
			}

		case "mesh:routing_update":
			var r struct {
				ParentTarget string `json:"parent_target"`
				ParentIsEdge bool   `json:"parent_is_edge"`
				Status       string `json:"status"`
			}
			if json.Unmarshal(raw, &r) == nil {
				n.mu.Lock()
				n.parent = r.ParentTarget
				n.parentIsEdge = r.ParentIsEdge
				n.routed = r.Status == "routed"
				n.mu.Unlock()
				n.logf("route -> parent=%s edge=%v status=%s", r.ParentTarget, r.ParentIsEdge, r.Status)
			}

		case "mesh:receive_rf":
			var pkt transmitFrame
			if json.Unmarshal(raw, &pkt) == nil {
				n.stats.received.Add(1)
				go n.handleReceive(ctx, pkt)
			}

		case "mesh:ack":
			var a struct {
				PacketID  string `json:"packet_id"`
				Duplicate bool   `json:"duplicate"`
			}
			if json.Unmarshal(raw, &a) == nil {
				n.mu.Lock()
				ch, ok := n.waiters[a.PacketID]
				if ok {
					delete(n.waiters, a.PacketID)
				}
				n.mu.Unlock()
				if ok {
					close(ch)
				}
			}

		case "session:ended":
			n.logf("session ended by server")
			return

		case "error":
			var e struct {
				Code, Message string
			}
			_ = json.Unmarshal(raw, &e)
			n.logf("server error: %s %s", e.Code, e.Message)
		}
	}
}

type transmitFrame struct {
	Event            string   `json:"event"`
	PacketID         string   `json:"packet_id"`
	OriginNode       string   `json:"origin_node"`
	TargetParent     string   `json:"target_parent"`
	HopCount         int      `json:"hop_count"`
	RoutingPath      []string `json:"routing_path"`
	BinaryPayloadB64 string   `json:"binary_payload_b64"`
}

// handleReceive is the store-and-forward half of the Data Link: ACK the
// sender, suppress duplicates, then relay the frame toward this ship's own
// parent.
func (n *node) handleReceive(ctx context.Context, pkt transmitFrame) {
	// Always ACK — real receivers acknowledge duplicates too.
	n.writeJSON(map[string]any{
		"event":         "node:ack",
		"packet_id":     pkt.PacketID,
		"receiver_node": n.id,
		"status":        "received",
	})

	n.mu.Lock()
	if n.seen[pkt.PacketID] {
		n.mu.Unlock()
		n.stats.dupSeen.Add(1)
		return
	}
	n.seen[pkt.PacketID] = true
	parent, routed := n.parent, n.routed
	n.mu.Unlock()

	if !routed || parent == "" {
		n.logf("received %s but I am isolated; packet stalls here", pkt.PacketID)
		return
	}

	fwd := pkt
	fwd.Event = "node:transmit"
	fwd.TargetParent = parent
	fwd.HopCount = pkt.HopCount + 1
	fwd.RoutingPath = append(append([]string{}, pkt.RoutingPath...), n.id)
	n.stats.forwarded.Add(1)
	n.transmitWithRetry(ctx, fwd)
}

// transmitTelemetry originates a fresh schema-shaped catch report.
func (n *node) transmitTelemetry(ctx context.Context) {
	n.mu.Lock()
	parent, routed, fields, maxPayload := n.parent, n.routed, n.fields, n.maxPayload
	n.pktSeq++
	seq := n.pktSeq
	n.mu.Unlock()

	if !routed || parent == "" {
		n.logf("skip transmit: no route")
		return
	}

	payload := n.buildPayload(fields)
	raw, err := msgpack.Marshal(payload)
	if err != nil {
		n.logf("msgpack marshal failed: %v", err)
		return
	}
	if len(raw) > maxPayload && !n.opt.ignoreSF {
		n.logf("payload %dB exceeds SF window %dB, skipping", len(raw), maxPayload)
		return
	}

	pkt := transmitFrame{
		Event:            "node:transmit",
		PacketID:         fmt.Sprintf("pkt-%s-%04d", n.id, seq),
		OriginNode:       n.id,
		TargetParent:     parent,
		HopCount:         1,
		RoutingPath:      []string{n.id},
		BinaryPayloadB64: base64.StdEncoding.EncodeToString(raw),
	}
	n.transmitWithRetry(ctx, pkt)
}

// transmitWithRetry implements the half-duplex Data Link contract: send,
// wait for mesh:ack up to the SF timeout, retry at most 3 times with the
// SAME packet id (that is what exercises the backend dedup), then declare
// the link dead.
func (n *node) transmitWithRetry(ctx context.Context, pkt transmitFrame) {
	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		n.mu.Lock()
		ch := make(chan struct{})
		n.waiters[pkt.PacketID] = ch
		timeout := n.ackTimeout
		n.mu.Unlock()

		if !n.writeJSON(pkt) {
			return
		}
		n.stats.sent.Add(1)
		if attempt > 0 {
			n.stats.retries.Add(1)
			n.logf("retry %d for %s", attempt, pkt.PacketID)
		}

		select {
		case <-ch:
			n.stats.acked.Add(1)
			n.logf("acked %s (attempt %d)", pkt.PacketID, attempt+1)
			return
		case <-ctx.Done():
			return
		case <-time.After(timeout + 500*time.Millisecond):
			n.mu.Lock()
			delete(n.waiters, pkt.PacketID)
			n.mu.Unlock()
		}
	}

	n.stats.failed.Add(1)
	n.logf("link dead for %s after %d retries; waiting for a new route", pkt.PacketID, maxRetries)
}

func (n *node) buildPayload(fields []schemaField) map[string]any {
	fishNames := []string{"tuna", "layur", "tongkol", "cakalang", "kembung", "tenggiri"}
	if len(fields) == 0 {
		return map[string]any{"lat": float32(n.lat), "lng": float32(n.lng), "berat_kg": uint16(rand.IntN(300) + 1)}
	}
	payload := make(map[string]any, len(fields))
	for _, f := range fields {
		switch {
		case f.Name == "lat" && strings.HasPrefix(f.Type, "float"):
			payload[f.Name] = float32(n.lat)
		case f.Name == "lng" && strings.HasPrefix(f.Type, "float"):
			payload[f.Name] = float32(n.lng)
		case strings.HasPrefix(f.Type, "float"):
			payload[f.Name] = float32(rand.Float64() * 100)
		case strings.HasPrefix(f.Type, "uint"), strings.HasPrefix(f.Type, "int"):
			payload[f.Name] = uint16(rand.IntN(500) + 1)
		case f.Type == "bool":
			payload[f.Name] = rand.IntN(2) == 0
		case strings.HasPrefix(f.Type, "string_"):
			name := fishNames[rand.IntN(len(fishNames))]
			var maxLen int
			fmt.Sscanf(f.Type, "string_%d", &maxLen)
			if maxLen > 0 && len(name) > maxLen {
				name = name[:maxLen]
			}
			payload[f.Name] = name
		}
	}
	return payload
}

// move advances the random walk: mostly straight, with gentle heading drift.
func (n *node) move() {
	stepKm := n.opt.speedKmh * n.opt.pingInterval.Hours()
	n.heading += (rand.Float64() - 0.5) * 0.6
	dLat := (stepKm * math.Cos(n.heading)) / 111.19
	dLng := (stepKm * math.Sin(n.heading)) / (111.19 * math.Cos(n.lat*math.Pi/180))
	n.lat += dLat
	n.lng += dLng
}

func (n *node) sendPing() {
	n.writeJSON(map[string]any{
		"event":      "node:ping",
		"node_id":    n.id,
		"session_id": n.sessionID,
		"lat":        n.lat,
		"lng":        n.lng,
	})
}

func (n *node) writeJSON(v any) bool {
	n.writeMu.Lock()
	defer n.writeMu.Unlock()
	_ = n.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := n.conn.WriteJSON(v); err != nil {
		return false
	}
	return true
}

func (n *node) writeControlClose() error {
	n.writeMu.Lock()
	defer n.writeMu.Unlock()
	return n.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"))
}

func (n *node) logf(format string, args ...any) {
	if n.opt.verbose {
		fmt.Printf("[%s] %s\n", n.id, fmt.Sprintf(format, args...))
	}
}

// --- helpers ------------------------------------------------------------------------

func nodeWSURL(api, sessionID, nodeID string) (string, error) {
	u, err := url.Parse(api)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	u.Path = "/ws/nodes"
	q := u.Query()
	q.Set("session_id", sessionID)
	q.Set("node_id", nodeID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// scatter places a ship uniformly inside a disk of radius km.
func scatter(lat, lng, radiusKm float64) (float64, float64) {
	r := radiusKm * math.Sqrt(rand.Float64())
	theta := rand.Float64() * 2 * math.Pi
	dLat := (r * math.Cos(theta)) / 111.19
	dLng := (r * math.Sin(theta)) / (111.19 * math.Cos(lat*math.Pi/180))
	return lat + dLat, lng + dLng
}

func printSummary(opt options, sessionID string, s *fleetStats) {
	fmt.Println("\n===== nodesim fleet summary =====")
	fmt.Printf("nodes:               %d\n", opt.nodes)
	fmt.Printf("frames sent:         %d (incl. retries)\n", s.sent.Load())
	fmt.Printf("acks received:       %d\n", s.acked.Load())
	fmt.Printf("retries:             %d\n", s.retries.Load())
	fmt.Printf("links declared dead: %d\n", s.failed.Load())
	fmt.Printf("frames relayed:      %d (store & forward)\n", s.forwarded.Load())
	fmt.Printf("rf frames received:  %d\n", s.received.Load())
	fmt.Printf("client-side dups:    %d\n", s.dupSeen.Load())
	fmt.Printf("\nserver-side truth:   curl %s/api/v1/simulations/%s/stats\n", opt.api, sessionID)
}
