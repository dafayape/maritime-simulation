'use client';

import L from 'leaflet';
import { memo, useMemo } from 'react';
import { Marker, Popup } from 'react-leaflet';

import { useTopologyStore } from '@/stores/topologyStore';

/**
 * Marker Virtual Edge (gateway Syahbandar). Jumlah edge sedikit dan posisinya
 * statis, sehingga divIcon DOM (lencana jangkar) aman dipakai di sini —
 * berbeda dengan marker kapal yang harus di kanvas. Lencana memakai ikon SVG
 * jangkar (bukan emoji) agar konsisten dengan set ikon `ui/icons.tsx` dan
 * tampil sama di semua platform; stroke gelap kontras di atas lingkaran cyan.
 */
const ANCHOR_SVG =
  '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#0f172a" ' +
  'stroke-width="2.25" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
  '<circle cx="12" cy="5" r="2"/><path d="M12 22V7"/><path d="M5 12H2a10 10 0 0 0 20 0h-3"/></svg>';

function EdgeMarkerInner({ edgeCode }: { edgeCode: string }) {
  const edge = useTopologyStore((s) => s.edges[edgeCode]);

  const icon = useMemo(
    () =>
      L.divIcon({
        className: '',
        html: `<div class="edge-marker">${ANCHOR_SVG}</div>`,
        iconSize: [26, 26],
        iconAnchor: [13, 13],
      }),
    [],
  );

  if (!edge) return null;

  return (
    <Marker position={[edge.latitude, edge.longitude]} icon={icon}>
      <Popup>
        <div className="space-y-1 text-xs">
          <p className="text-sm font-semibold">{edge.name}</p>
          <p className="text-slate-300">
            Virtual Edge <span className="font-mono">{edge.edge_code}</span>
          </p>
          <p className="text-slate-300">
            {edge.latitude.toFixed(5)}, {edge.longitude.toFixed(5)}
          </p>
        </div>
      </Popup>
    </Marker>
  );
}

export const EdgeMarker = memo(EdgeMarkerInner);
