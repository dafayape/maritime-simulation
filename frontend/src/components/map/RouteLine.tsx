'use client';

import L from 'leaflet';
import { memo, useMemo } from 'react';
import { Marker, Polyline } from 'react-leaflet';

import { ROUTE_COLOR } from '@/components/map/mapConstants';
import { bearingDeg, midpoint } from '@/lib/geo';
import { useTopologyStore } from '@/stores/topologyStore';

/**
 * Satu ruas rute mesh: Polyline putus-putus dari kapal ke parent-nya
 * ditambah ikon panah arah di titik tengah (SRS §3A.2).
 *
 * Panah memakai divIcon DOM kecil yang dirotasi CSS sesuai bearing —
 * trade-off yang disengaja: plugin decorator polyline pihak ketiga sudah
 * tidak terawat, sedangkan ~50 elemen DOM statis per tick jauh di bawah
 * ambang masalah performa (berbeda dengan marker kapal yang bergerak tiap
 * detik, sehingga tetap di kanvas).
 */
function RouteLineInner({ nodeId }: { nodeId: string }) {
  const node = useTopologyStore((s) => s.nodes[nodeId]);
  const parentNode = useTopologyStore((s) => {
    const n = s.nodes[nodeId];
    return n && n.parent && !n.parentIsEdge ? s.nodes[n.parent] : undefined;
  });
  const parentEdge = useTopologyStore((s) => {
    const n = s.nodes[nodeId];
    return n && n.parent && n.parentIsEdge ? s.edges[n.parent] : undefined;
  });

  const from: [number, number] | null =
    node && Number.isFinite(node.lat) && Number.isFinite(node.lng)
      ? [node.lat, node.lng]
      : null;

  let to: [number, number] | null = null;
  if (parentEdge) {
    to = [parentEdge.latitude, parentEdge.longitude];
  } else if (parentNode && Number.isFinite(parentNode.lat) && Number.isFinite(parentNode.lng)) {
    to = [parentNode.lat, parentNode.lng];
  }

  const arrowIcon = useMemo(() => {
    if (!from || !to) return null;
    const bearing = bearingDeg(from[0], from[1], to[0], to[1]);
    return L.divIcon({
      className: '',
      html: `<div class="route-arrow" style="transform: rotate(${bearing.toFixed(1)}deg)"></div>`,
      iconSize: [10, 10],
      iconAnchor: [5, 5],
    });
    // from/to adalah tuple baru tiap render; dependensi nilai primitifnya.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [from?.[0], from?.[1], to?.[0], to?.[1]]);

  if (!node || node.status !== 'routed' || !from || !to || !arrowIcon) return null;

  const mid = midpoint(from[0], from[1], to[0], to[1]);

  return (
    <>
      <Polyline
        positions={[from, to]}
        pathOptions={{
          color: ROUTE_COLOR,
          weight: 2,
          opacity: 0.75,
          dashArray: '6 6',
        }}
      />
      <Marker position={mid} icon={arrowIcon} interactive={false} keyboard={false} />
    </>
  );
}

export const RouteLine = memo(RouteLineInner);
