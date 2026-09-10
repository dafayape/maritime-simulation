-- 0001_init.sql
-- Maritime LoRa Mesh Network Simulator - initial schema (SRS §2A).
-- Note: the SRS was written with MySQL-flavoured types (BIGINT UNSIGNED);
-- this is the PostgreSQL 16 adaptation (BIGSERIAL / TIMESTAMPTZ / JSONB).

CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(150) NOT NULL,
    email         VARCHAR(150) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS virtual_edges (
    id         BIGSERIAL PRIMARY KEY,
    edge_code  VARCHAR(50) NOT NULL UNIQUE,
    name       VARCHAR(150) NOT NULL,
    latitude   DECIMAL(11, 8) NOT NULL,
    longitude  DECIMAL(11, 8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS simulation_sessions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_name     VARCHAR(150) NOT NULL,
    spreading_factor INT NOT NULL DEFAULT 7,
    tx_power_dbm     INT NOT NULL DEFAULT 20,
    weather_severity DECIMAL(3, 2) NOT NULL DEFAULT 1.00,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    -- Deviation from SRS (documented in README): final packet-loss / retry
    -- statistics are snapshotted here when a session is stopped, because the
    -- PRD requires persisting simulation statistics but the SRS schema gave
    -- them no home.
    stats_snapshot   JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at         TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS dynamic_schemas (
    id                BIGSERIAL PRIMARY KEY,
    session_id        UUID NOT NULL REFERENCES simulation_sessions (id) ON DELETE CASCADE,
    schema_definition JSONB NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dynamic_schemas_session
    ON dynamic_schemas (session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS telemetry_logs (
    id              BIGSERIAL PRIMARY KEY,
    session_id      UUID NOT NULL REFERENCES simulation_sessions (id) ON DELETE CASCADE,
    origin_node_id  VARCHAR(100) NOT NULL,
    edge_id         BIGINT REFERENCES virtual_edges (id) ON DELETE SET NULL,
    hop_count       INT NOT NULL,
    routing_path    VARCHAR(255),
    decoded_payload JSONB,
    arrived_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_logs_arrived_at
    ON telemetry_logs (arrived_at);

CREATE INDEX IF NOT EXISTS idx_telemetry_logs_session
    ON telemetry_logs (session_id, arrived_at DESC);

-- Seed one Virtual Edge (Syahbandar Pelabuhan Ratu) so a fresh deployment is
-- immediately usable; more edges can be registered via POST /api/v1/edges.
INSERT INTO virtual_edges (edge_code, name, latitude, longitude)
VALUES ('EDGE-PRATU-01', 'Syahbandar Pelabuhan Ratu', -6.98750000, 106.55040000)
ON CONFLICT (edge_code) DO NOTHING;
