'use client';

import { memo, useCallback, useState } from 'react';
import { CircleMarker, Popup } from 'react-leaflet';

import { NODE_COLOR } from '@/components/map/mapConstants';
import { listTelemetry } from '@/lib/api';
import { formatKm } from '@/lib/format';
import { useSimulationStore } from '@/stores/simulationStore';
import { useTopologyStore, type ShipNode } from '@/stores/topologyStore';
import type { TelemetryLog } from '@/types/backend';

function markerColor(node: ShipNode): string {
  if (!node.online) return NODE_COLOR.offline;
  if (node.status === 'isolated') return NODE_COLOR.isolated;
  if (node.linkState === 'retrying') return NODE_COLOR.retrying;
  return NODE_COLOR.connected;
}

function statusLabel(node: ShipNode): string {
  if (!node.online) return 'Offline — kapal terputus';
  if (node.status === 'isolated') return 'Terisolasi — tanpa rute';
  if (node.linkState === 'retrying') return 'Retrying — menunggu ACK';
  return `Terhubung · hop ${node.hopLevel}`;
}

type PayloadFetch =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'done'; log: TelemetryLog | null }
  | { state: 'error'; message: string };

/**
 * Satu titik kapal (SRS §3A.1) — CircleMarker kanvas, BUKAN <Marker> berbasis
 * ikon DOM: dengan renderer Canvas (preferCanvas di MapContainer) 50+ titik
 * yang bergerak digambar ulang pada satu <canvas>, jauh lebih murah daripada
 * memindahkan 50+ elemen DOM. Trade-off: bentuk marker terbatas lingkaran —
 * dapat diterima karena identitas visual cukup lewat warna status.
 *
 * Komponen ber-subscribe ke node-nya SENDIRI via selector Zustand, sehingga
 * pergerakan kapal lain tidak pernah me-render ulang marker ini (SRS §3B).
 */
function ShipMarkerInner({ nodeId }: { nodeId: string }) {
  const node = useTopologyStore((s) => s.nodes[nodeId]);
  const sessionId = useSimulationStore((s) => s.sessionId);
  const [payload, setPayload] = useState<PayloadFetch>({ state: 'idle' });

  // Payload terakhir diambil on-demand saat popup dibuka (event klik user,
  // bukan polling): endpoint telemetry belum punya filter per-node, jadi
  // halaman terakhir diambil lalu disaring origin_node_id di sisi klien.
  const loadLastPayload = useCallback(() => {
    if (!sessionId) return;
    setPayload({ state: 'loading' });
    listTelemetry(sessionId, 100, 0)
      .then((res) => {
        const log = (res.items ?? []).find((t) => t.origin_node_id === nodeId) ?? null;
        setPayload({ state: 'done', log });
      })
      .catch((err: Error) => setPayload({ state: 'error', message: err.message }));
  }, [sessionId, nodeId]);

  if (!node || !Number.isFinite(node.lat) || !Number.isFinite(node.lng)) return null;

  const color = markerColor(node);

  return (
    <CircleMarker
      center={[node.lat, node.lng]}
      radius={7}
      pathOptions={{
        color: '#0f172a',
        weight: 1.5,
        fillColor: color,
        fillOpacity: node.online ? 0.95 : 0.35,
        opacity: node.online ? 1 : 0.4,
      }}
      eventHandlers={{ popupopen: loadLastPayload }}
    >
      <Popup maxWidth={280}>
        <div className="min-w-56 space-y-1.5 text-xs">
          <div className="flex items-center gap-2">
            <span className="inline-block h-2.5 w-2.5 rounded-full" style={{ background: color }} />
            <span className="text-sm font-semibold">{node.id}</span>
            {!node.online && <span className="text-[10px] text-slate-300">(offline)</span>}
          </div>
          <p className="text-slate-300">{statusLabel(node)}</p>
          <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 text-slate-300">
            <dt>Parent</dt>
            <dd className="text-slate-200">
              {node.parent ?? '—'}
              {node.parent && node.parentIsEdge && ' (Edge)'}
            </dd>
            <dt>Jarak</dt>
            <dd className="text-slate-200">{node.parent ? formatKm(node.distanceKm) : '—'}</dd>
            <dt>Posisi</dt>
            <dd className="text-slate-200">
              {node.lat.toFixed(5)}, {node.lng.toFixed(5)}
            </dd>
          </dl>
          <div className="border-t border-white/10 pt-1.5">
            <p className="mb-1 font-semibold text-slate-300">Payload terakhir sampai Edge</p>
            {payload.state === 'loading' && <p className="text-slate-300">Memuat…</p>}
            {payload.state === 'error' && <p className="text-rose-300">{payload.message}</p>}
            {payload.state === 'done' && !payload.log && (
              <p className="text-slate-300">Belum ada paket dari kapal ini yang mendarat.</p>
            )}
            {payload.state === 'done' && payload.log && (
              <div className="space-y-1">
                <div className="flex flex-wrap gap-1">
                  {Object.entries(payload.log.decoded_payload ?? {}).map(([k, v]) => (
                    <span key={k} className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px]">
                      {k}: <span className="text-emerald-300">{String(v)}</span>
                    </span>
                  ))}
                </div>
                <p className="text-[10px] text-slate-300">
                  hop {payload.log.hop_count} · {payload.log.routing_path}
                </p>
              </div>
            )}
          </div>
        </div>
      </Popup>
    </CircleMarker>
  );
}

export const ShipMarker = memo(ShipMarkerInner);
