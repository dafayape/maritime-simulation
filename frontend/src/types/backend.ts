/**
 * Mirror tipe data backend Go (SRS Frontend §7.3 "Type Safety").
 *
 * Setiap interface di file ini merepresentasikan struct Go pada
 * `backend/internal/{domain,protocol,service,api}` secara presisi, termasuk
 * nama field JSON snake_case-nya. Jangan menambah/mengubah field tanpa
 * mencocokkan dengan tag `json:"..."` pada struct Go aslinya.
 *
 * Catatan penting: Go meng-encode slice `nil` sebagai `null` (bukan `[]`),
 * sehingga semua field array dari backend bertipe `T[] | null`.
 */

// --- REST envelope (backend/internal/api/response.go) -----------------------

export interface ApiEnvelope<T> {
  status: 'success' | 'error';
  message: string;
  data: T;
}

// --- Entitas (backend/internal/domain/entities.go) --------------------------

export interface Session {
  id: string;
  session_name: string;
  spreading_factor: number;
  tx_power_dbm: number;
  weather_severity: number;
  is_active: boolean;
  stats_snapshot?: Record<string, number>;
  created_at: string;
  ended_at?: string;
}

export interface Edge {
  id: number;
  edge_code: string;
  name: string;
  latitude: number;
  longitude: number;
  created_at: string;
  updated_at: string;
}

export interface TelemetryLog {
  id: number;
  session_id: string;
  origin_node_id: string;
  edge_id?: number;
  /** Kode edge di-snapshot saat kedatangan paket; bertahan meski edge itu
   *  kemudian dihapus dari master data (lihat `edge_active`). */
  edge_code?: string;
  /** true = edge dengan kode ini masih terdaftar hari ini (hijau di UI);
   *  false = sudah dihapus dari master data, nama tetap tampil (merah). */
  edge_active: boolean;
  hop_count: number;
  routing_path: string;
  decoded_payload: Record<string, unknown> | null;
  arrived_at: string;
}

// --- Mesh routing (backend/internal/domain/mesh.go) --------------------------

export type RouteStatus = 'routed' | 'isolated';

export interface RouteEntry {
  node: string;
  parent: string;
  parent_is_edge: boolean;
  distance_km: number;
  hop_level: number;
  status: RouteStatus;
}

// --- Skema dinamis (backend/internal/domain/schema.go) -----------------------

export interface SchemaField {
  name: string;
  type: string;
}

export interface DynamicSchema {
  id: number;
  session_id: string;
  fields: SchemaField[] | null;
  created_at: string;
}

// --- Snapshot topologi (backend/internal/service/mesh_topology.go) -----------

export interface TopologyNode {
  node_id: string;
  lat: number;
  lng: number;
  online: boolean;
}

export interface TopologySnapshot {
  nodes: TopologyNode[] | null;
  routes: RouteEntry[] | null;
  edges: Edge[] | null;
}

// --- Statistik live (backend/internal/service/stats.go) ----------------------

/** Peta nama counter Redis -> nilai. Nama kanonik lihat STAT_* di lib/lora. */
export type LiveStats = Record<string, number>;

// --- Payload REST spesifik ----------------------------------------------------

export interface CreateSimulationRequest {
  session_name: string;
  spreading_factor?: number;
  tx_power_dbm?: number;
  weather_severity?: number;
}

export interface CreateSimulationResponse {
  session_id: string;
  session: Session;
}

export interface GetSimulationResponse {
  session: Session;
  live_stats: LiveStats | null;
}

export interface SetSchemaResponse {
  schema: DynamicSchema;
  estimated_payload_bytes: number;
  warning: string;
}

export interface TelemetryListResponse {
  items: TelemetryLog[] | null;
  total: number;
  limit: number;
  offset: number;
}

export interface CreateEdgeRequest {
  edge_code: string;
  name: string;
  latitude: number;
  longitude: number;
}

/** edge_code tidak disertakan — bersifat immutable, dikirim lewat path URL. */
export interface UpdateEdgeRequest {
  name: string;
  latitude: number;
  longitude: number;
}

// --- Event WebSocket monitor (backend/internal/protocol/events.go) -----------
// Kanal dasbor adalah `GET /ws/monitor?session_id=...` (read-only).

export interface NodePingEvent {
  event: 'node:ping';
  node_id: string;
  session_id: string;
  lat: number;
  lng: number;
}

/**
 * Varian monitor dari mesh:routing_update: membawa SELURUH tabel routing
 * (protocol.RoutingTable), bukan direktif per-node yang diterima kapal.
 */
export interface RoutingTableEvent {
  event: 'mesh:routing_update';
  routes: RouteEntry[] | null;
}

export interface EnvSyncParamsEvent {
  event: 'env:sync_params';
  sf: number;
  max_payload_bytes: number;
  ack_timeout_ms: number;
  max_retries: number;
  tx_power_dbm: number;
  max_range_km: number;
  weather_severity: number;
}

export interface SchemaSyncEvent {
  event: 'schema:sync';
  fields: SchemaField[] | null;
  estimated_packed_bytes: number;
}

export type PacketEventType =
  | 'transmit'
  | 'forward'
  | 'deliver'
  | 'drop'
  | 'duplicate'
  | 'ack'
  | 'ack_drop';

export interface PacketEvent {
  event: 'packet:event';
  type: PacketEventType;
  packet_id: string;
  from_node: string;
  to_node: string;
  origin_node?: string;
  hop_count?: number;
  distance_km?: number;
  loss_probability?: number;
  reason?: string;
  at: string;
}

export interface NodeStatusEvent {
  event: 'node:status';
  node_id: string;
  online: boolean;
}

export interface SessionEndedEvent {
  event: 'session:ended';
  session_id: string;
  message: string;
}

export interface WsErrorEvent {
  event: 'error';
  code: string;
  message: string;
}

export type MonitorEvent =
  | NodePingEvent
  | RoutingTableEvent
  | EnvSyncParamsEvent
  | SchemaSyncEvent
  | PacketEvent
  | NodeStatusEvent
  | SessionEndedEvent
  | WsErrorEvent;
