'use client';

import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import { useShallow } from 'zustand/react/shallow';

import { useTopologyStore } from '@/stores/topologyStore';
import { useUiStore } from '@/stores/uiStore';

/**
 * Sekali saja setelah data pertama tiba: pas-kan kamera ke seluruh armada +
 * edge. Setelah itu kamera diserahkan penuh ke user (tidak ada auto-pan yang
 * merebut kendali saat kapal bergerak).
 */
export function AutoFitOnce() {
  const map = useMap();
  const done = useRef(false);
  const points = useTopologyStore(
    useShallow((s) => {
      const pts: Array<[number, number]> = [];
      for (const n of Object.values(s.nodes)) {
        if (Number.isFinite(n.lat) && Number.isFinite(n.lng)) pts.push([n.lat, n.lng]);
      }
      for (const e of Object.values(s.edges)) {
        pts.push([e.latitude, e.longitude]);
      }
      // Selector mengembalikan jumlah titik saja: cukup untuk mendeteksi
      // "data pertama tiba" tanpa memicu re-render tiap pergeseran koordinat.
      return pts.length;
    }),
  );

  useEffect(() => {
    if (done.current || points === 0) return;
    const { nodes, edges } = useTopologyStore.getState();
    const pts: Array<[number, number]> = [];
    for (const n of Object.values(nodes)) {
      if (Number.isFinite(n.lat) && Number.isFinite(n.lng)) pts.push([n.lat, n.lng]);
    }
    for (const e of Object.values(edges)) {
      pts.push([e.latitude, e.longitude]);
    }
    if (pts.length === 0) return;
    done.current = true;
    map.fitBounds(pts, { padding: [48, 48], maxZoom: 12 });
  }, [map, points]);

  return null;
}

/** Terbangkan kamera ke kapal yang diklik pada panel daftar kapal. */
export function FocusFlyer() {
  const map = useMap();
  const requestId = useUiStore((s) => s.focusRequestId);

  useEffect(() => {
    if (requestId === 0) return;
    const nodeId = useUiStore.getState().focusNodeId;
    if (!nodeId) return;
    const node = useTopologyStore.getState().nodes[nodeId];
    if (!node || !Number.isFinite(node.lat) || !Number.isFinite(node.lng)) return;
    map.flyTo([node.lat, node.lng], Math.max(map.getZoom(), 13), { duration: 0.8 });
  }, [map, requestId]);

  return null;
}
