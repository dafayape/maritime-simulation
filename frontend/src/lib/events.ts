/**
 * Router event WebSocket monitor -> store Zustand.
 *
 * Dipisah dari hook React sebagai fungsi murni terhadap antarmuka `EventSinks`
 * supaya (1) dapat diunit-test tanpa DOM/WebSocket, dan (2) pemrosesan pesan
 * berjalan lewat store.getState() di luar siklus render React — penting saat
 * ratusan event per detik masuk ketika badai broadcast.
 */

import { weatherLabel } from '@/lib/format';
import type { MonitorEvent, PacketEvent, RouteEntry, SchemaField } from '@/types/backend';

/** Nama event kontrak backend (internal/protocol/events.go). */
export const EV = {
  nodePing: 'node:ping',
  routingUpdate: 'mesh:routing_update',
  envSyncParams: 'env:sync_params',
  schemaSync: 'schema:sync',
  packetEvent: 'packet:event',
  nodeStatus: 'node:status',
  sessionEnded: 'session:ended',
  error: 'error',
} as const;

const KNOWN_EVENTS = new Set<string>(Object.values(EV));

/** Parse satu frame WS; null bila bukan JSON ber-discriminator yang dikenal. */
export function parseMonitorEvent(raw: string): MonitorEvent | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  if (typeof parsed !== 'object' || parsed === null) return null;
  const event = (parsed as { event?: unknown }).event;
  if (typeof event !== 'string' || !KNOWN_EVENTS.has(event)) return null;
  return parsed as MonitorEvent;
}

/**
 * Sink minimal yang dibutuhkan router — subset dari API store asli, dibuat
 * struktural agar unit test cukup menyuplai objek polos.
 */
export interface EventSinks {
  topology: {
    applyPing(nodeId: string, lat: number, lng: number): void;
    applyRoutingTable(routes: RouteEntry[] | null): void;
    setOnline(nodeId: string, online: boolean): void;
    applyPacketEvent(ev: PacketEvent): void;
  };
  metrics: {
    applyPacketEvent(ev: PacketEvent): void;
  };
  logs: {
    appendPacket(ev: PacketEvent): void;
    appendSystem(text: string, level?: 'info' | 'warn' | 'error'): void;
  };
  simulation: {
    setEnvParams(params: Extract<MonitorEvent, { event: 'env:sync_params' }>): void;
    setSchema(fields: SchemaField[] | null, estimatedBytes: number): void;
    markEnded(message: string): void;
  };
  /** Opsional (murni visual): partikel aliran data saat paket berhasil hop. */
  flow?: {
    spawnFromPacket(ev: PacketEvent): void;
  };
}

/** Terapkan satu event monitor ke seluruh store terkait. */
export function routeMonitorEvent(ev: MonitorEvent, sinks: EventSinks): void {
  switch (ev.event) {
    case EV.nodePing:
      sinks.topology.applyPing(ev.node_id, ev.lat, ev.lng);
      return;

    case EV.routingUpdate:
      sinks.topology.applyRoutingTable(ev.routes);
      return;

    case EV.envSyncParams:
      sinks.simulation.setEnvParams(ev);
      sinks.logs.appendSystem(
        `Parameter radio: SF${ev.sf} · ${ev.tx_power_dbm} dBm (~${ev.max_range_km.toFixed(1)} km) · cuaca ${ev.weather_severity.toFixed(1)} (${weatherLabel(ev.weather_severity)})`,
      );
      return;

    case EV.schemaSync:
      sinks.simulation.setSchema(ev.fields, ev.estimated_packed_bytes);
      sinks.logs.appendSystem(
        `Skema payload aktif: ${(ev.fields ?? []).length} field · ±${ev.estimated_packed_bytes} byte`,
      );
      return;

    case EV.packetEvent:
      sinks.metrics.applyPacketEvent(ev);
      sinks.topology.applyPacketEvent(ev);
      sinks.logs.appendPacket(ev);
      sinks.flow?.spawnFromPacket(ev);
      return;

    case EV.nodeStatus:
      sinks.topology.setOnline(ev.node_id, ev.online);
      sinks.logs.appendSystem(
        ev.online ? `Kapal ${ev.node_id} bergabung` : `Kapal ${ev.node_id} terputus`,
        ev.online ? 'info' : 'warn',
      );
      return;

    case EV.sessionEnded:
      sinks.simulation.markEnded(ev.message);
      sinks.logs.appendSystem(`Sesi dihentikan: ${ev.message}`, 'warn');
      return;

    case EV.error:
      sinks.logs.appendSystem(`Backend menolak frame: [${ev.code}] ${ev.message}`, 'error');
      return;

    default:
      // Event baru dari backend versi lebih baru diabaikan diam-diam agar
      // dasbor lama tetap kompatibel ke depan.
      return;
  }
}
