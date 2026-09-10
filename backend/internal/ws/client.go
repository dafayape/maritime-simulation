package ws

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait bounds a single frame write.
	writeWait = 10 * time.Second
	// pongWait is how long a silent peer stays alive; pings go out at
	// pingPeriod (< pongWait) to keep healthy peers marked live.
	pongWait   = 60 * time.Second
	pingPeriod = 50 * time.Second
	// maxMessageSize caps inbound frames. The largest legal payload is a
	// 242-byte SF7 frame -> ~330 bytes of Base64 inside a small JSON
	// envelope; 16 KiB leaves generous headroom while still stopping abuse.
	maxMessageSize = 16 * 1024
)

type clientKind int

const (
	kindNode clientKind = iota
	kindMonitor
)

// Client is one live socket. Outbound traffic flows through a buffered
// channel drained by writePump, so a slow consumer can never block the
// simulation engine (frames to it are dropped instead — like a saturated
// radio).
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	log  *slog.Logger

	kind      clientKind
	sessionID string
	nodeID    string // empty for monitors

	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func newClient(hub *Hub, conn *websocket.Conn, kind clientKind, sessionID, nodeID string, log *slog.Logger) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		log:       log,
		kind:      kind,
		sessionID: sessionID,
		nodeID:    nodeID,
		send:      make(chan []byte, hub.sendBuffer),
		done:      make(chan struct{}),
	}
}

// trySend queues a frame without ever blocking. Returns false when the
// client is gone or its buffer is full.
func (c *Client) trySend(raw []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- raw:
		return true
	default:
		c.log.Warn("send buffer full, frame dropped",
			"session_id", c.sessionID, "node_id", c.nodeID)
		return false
	}
}

// Close is idempotent and unblocks both pumps.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// readPump consumes inbound frames and hands them to onMessage. It owns the
// connection teardown: when it returns, the socket is closed and onClose
// runs exactly once. Every SRS §6.1 requirement lives here: deferred close,
// context awareness and panic recovery.
func (c *Client) readPump(ctx context.Context, onMessage func(raw []byte), onClose func()) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("websocket read pump panicked",
				"session_id", c.sessionID, "node_id", c.nodeID,
				"panic", r, "stack", string(debug.Stack()))
		}
		c.Close()
		_ = c.conn.Close()
		onClose()
	}()

	// Cancel the blocking ReadMessage when the server shuts down or the
	// writer decides the peer is dead.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
		case <-c.done:
		case <-stop:
			return
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseNoStatusReceived) {
				c.log.Debug("websocket closed unexpectedly",
					"session_id", c.sessionID, "node_id", c.nodeID, "error", err.Error())
			}
			return
		}
		onMessage(raw)
	}
}

// writePump drains the send channel and keeps the peer alive with pings.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("websocket write pump panicked",
				"session_id", c.sessionID, "node_id", c.nodeID,
				"panic", r, "stack", string(debug.Stack()))
		}
		ticker.Stop()
		c.Close()
		_ = c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
			return
		case raw := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
