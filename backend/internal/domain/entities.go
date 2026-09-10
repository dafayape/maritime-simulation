package domain

import "time"

// Session mirrors the simulation_sessions table (SRS §2A.3).
type Session struct {
	ID              string         `json:"id"`
	SessionName     string         `json:"session_name"`
	SpreadingFactor int            `json:"spreading_factor"`
	TxPowerDbm      int            `json:"tx_power_dbm"`
	WeatherSeverity float64        `json:"weather_severity"`
	IsActive        bool           `json:"is_active"`
	StatsSnapshot   map[string]any `json:"stats_snapshot,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	EndedAt         *time.Time     `json:"ended_at,omitempty"`
}

// Edge mirrors the virtual_edges table (SRS §2A.2) — a Syahbandar endpoint.
type Edge struct {
	ID        int64     `json:"id"`
	EdgeCode  string    `json:"edge_code"`
	Name      string    `json:"name"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TelemetryLog mirrors the telemetry_logs table (SRS §2A.5): one row per
// packet that survived the mesh and arrived at a Virtual Edge.
type TelemetryLog struct {
	ID             int64          `json:"id"`
	SessionID      string         `json:"session_id"`
	OriginNodeID   string         `json:"origin_node_id"`
	EdgeID       *int64 `json:"edge_id,omitempty"`
	EdgeCode     string `json:"edge_code,omitempty"`
	// EdgeActive reports whether a Virtual Edge with EdgeCode still exists in
	// master data. EdgeCode is a snapshot frozen at delivery time, so it
	// survives edge deletion; EdgeActive is what tells the dashboard the
	// difference between "delivered to a still-registered Syahbandar" (true)
	// and "delivered to a Syahbandar since removed" (false) — rendered as
	// green vs. red badges in the Reports table.
	EdgeActive bool `json:"edge_active"`
	HopCount   int  `json:"hop_count"`
	RoutingPath    string         `json:"routing_path"`
	DecodedPayload map[string]any `json:"decoded_payload"`
	ArrivedAt      time.Time      `json:"arrived_at"`
}
