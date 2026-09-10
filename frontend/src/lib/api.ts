/**
 * Klien REST transaksional (PRD §3.4): Axios + pembongkar envelope
 * `{ status, message, data }` yang diwajibkan backend (SRS Backend §3).
 *
 * Semua fungsi melempar ApiError dengan pesan asli backend agar form UI
 * dapat menampilkan alasan validasi persis dari domain Go.
 */

import axios, { AxiosError, type AxiosInstance, type AxiosResponse } from 'axios';

import { apiBaseUrl } from '@/lib/config';
import type {
  ApiEnvelope,
  CreateEdgeRequest,
  CreateSimulationRequest,
  CreateSimulationResponse,
  DynamicSchema,
  Edge,
  GetSimulationResponse,
  LiveStats,
  SchemaField,
  Session,
  SetSchemaResponse,
  TelemetryListResponse,
  TopologySnapshot,
  UpdateEdgeRequest,
} from '@/types/backend';

export class ApiError extends Error {
  constructor(
    message: string,
    readonly httpStatus?: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// Instance dibuat malas (lazy) karena base URL bisa bergantung pada
// window.location (mode same-origin) yang belum ada saat modul dievaluasi
// di server bundle.
let client: AxiosInstance | null = null;

function http(): AxiosInstance {
  if (!client) {
    client = axios.create({
      baseURL: apiBaseUrl(),
      timeout: 10_000,
      headers: { 'Content-Type': 'application/json' },
    });
  }
  return client;
}

async function unwrap<T>(request: Promise<AxiosResponse<ApiEnvelope<T>>>): Promise<T> {
  try {
    const res = await request;
    if (res.data.status !== 'success') {
      throw new ApiError(res.data.message, res.status);
    }
    return res.data.data;
  } catch (err) {
    if (err instanceof ApiError) throw err;
    const ax = err as AxiosError<ApiEnvelope<unknown>>;
    const message = ax.response?.data?.message ?? ax.message ?? 'gagal menghubungi backend';
    throw new ApiError(message, ax.response?.status);
  }
}

// --- Lifecycle simulasi -------------------------------------------------------

export function createSimulation(req: CreateSimulationRequest): Promise<CreateSimulationResponse> {
  return unwrap(http().post<ApiEnvelope<CreateSimulationResponse>>('/api/v1/simulations', req));
}

export function listSimulations(limit = 50): Promise<Session[] | null> {
  return unwrap(
    http().get<ApiEnvelope<Session[] | null>>('/api/v1/simulations', { params: { limit } }),
  );
}

export function getSimulation(id: string): Promise<GetSimulationResponse> {
  return unwrap(http().get<ApiEnvelope<GetSimulationResponse>>(`/api/v1/simulations/${id}`));
}

export function stopSimulation(id: string): Promise<Session> {
  return unwrap(http().post<ApiEnvelope<Session>>(`/api/v1/simulations/${id}/stop`, {}));
}

/** Hapus riwayat sesi (baris sesi + skema + telemetri, di-cascade backend).
 *  Backend menolak (409) bila sesi masih aktif — hentikan dulu sesinya. */
export function deleteSession(id: string): Promise<{ session_id: string }> {
  return unwrap(
    http().delete<ApiEnvelope<{ session_id: string }>>(`/api/v1/simulations/${id}`),
  );
}

export function updateWeather(id: string, weatherSeverity: number): Promise<Session> {
  return unwrap(
    http().put<ApiEnvelope<Session>>(`/api/v1/simulations/${id}/weather`, {
      weather_severity: weatherSeverity,
    }),
  );
}

export function updateRadioParams(
  id: string,
  params: { spreading_factor?: number; tx_power_dbm?: number },
): Promise<Session> {
  return unwrap(http().put<ApiEnvelope<Session>>(`/api/v1/simulations/${id}/params`, params));
}

// --- Skema dinamis --------------------------------------------------------------

export function setSchema(id: string, fields: SchemaField[]): Promise<SetSchemaResponse> {
  return unwrap(
    http().post<ApiEnvelope<SetSchemaResponse>>(`/api/v1/simulations/${id}/schema`, { fields }),
  );
}

export function getSchema(id: string): Promise<DynamicSchema> {
  return unwrap(http().get<ApiEnvelope<DynamicSchema>>(`/api/v1/simulations/${id}/schema`));
}

// --- Pembacaan dasbor ------------------------------------------------------------

export function getTopology(id: string): Promise<TopologySnapshot> {
  return unwrap(http().get<ApiEnvelope<TopologySnapshot>>(`/api/v1/simulations/${id}/topology`));
}

export function getStats(id: string): Promise<LiveStats> {
  return unwrap(http().get<ApiEnvelope<LiveStats>>(`/api/v1/simulations/${id}/stats`));
}

export function listTelemetry(
  id: string,
  limit = 50,
  offset = 0,
): Promise<TelemetryListResponse> {
  return unwrap(
    http().get<ApiEnvelope<TelemetryListResponse>>(`/api/v1/simulations/${id}/telemetry`, {
      params: { limit, offset },
    }),
  );
}

// --- Virtual edges ----------------------------------------------------------------

export function listEdges(): Promise<Edge[] | null> {
  return unwrap(http().get<ApiEnvelope<Edge[] | null>>('/api/v1/edges'));
}

export function createEdge(req: CreateEdgeRequest): Promise<Edge> {
  return unwrap(http().post<ApiEnvelope<Edge>>('/api/v1/edges', req));
}

/** Ubah nama/koordinat edge. edge_code tetap (kunci routing), sehingga rute
 *  aktif yang sudah memakainya sebagai target tidak terganggu. */
export function updateEdge(code: string, req: UpdateEdgeRequest): Promise<Edge> {
  return unwrap(
    http().put<ApiEnvelope<Edge>>(`/api/v1/edges/${encodeURIComponent(code)}`, req),
  );
}

/** Hapus Virtual Edge berdasarkan edge_code. Backend memutus referensi
 *  telemetri (ON DELETE SET NULL) sehingga riwayat tetap utuh. */
export function deleteEdge(code: string): Promise<{ edge_code: string }> {
  return unwrap(
    http().delete<ApiEnvelope<{ edge_code: string }>>(
      `/api/v1/edges/${encodeURIComponent(code)}`,
    ),
  );
}

// --- Health -----------------------------------------------------------------------

export async function backendHealthy(): Promise<boolean> {
  try {
    await http().get('/healthz', { timeout: 4_000 });
    return true;
  } catch {
    return false;
  }
}
