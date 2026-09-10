'use client';

import L from 'leaflet';
import { memo, useEffect, useMemo } from 'react';
import { Marker, useMap } from 'react-leaflet';
import { useShallow } from 'zustand/react/shallow';

import { useFlowStore, type Flight } from '@/stores/flowStore';

/** Durasi tempuh satu partikel (selaras --flow-ms & keyframes di globals.css). */
const FLOW_MS = 850;

/**
 * Satu partikel data. Ditempatkan sebagai Marker di titik ASAL; delta piksel
 * ke tujuan dihitung sekali saat spawn (proyeksi layar pada zoom saat itu) lalu
 * dituang ke CSS var --dx/--dy, sehingga animasi CSS-lah yang menggerakkan
 * div-nya — tanpa update latlng per-frame (murah). Flight berumur pendek jadi
 * pergeseran akibat zoom/pan di tengah jalan dapat diabaikan.
 */
function FlowDot({ flight }: { flight: Flight }) {
  const map = useMap();
  const remove = useFlowStore((s) => s.remove);

  const icon = useMemo(() => {
    const from = map.latLngToLayerPoint([flight.fromLat, flight.fromLng]);
    const to = map.latLngToLayerPoint([flight.toLat, flight.toLng]);
    const dx = (to.x - from.x).toFixed(1);
    const dy = (to.y - from.y).toFixed(1);
    const cls = flight.kind === 'deliver' ? 'flow-dot flow-deliver' : 'flow-dot';
    return L.divIcon({
      className: '',
      html: `<div class="${cls}" style="--dx:${dx}px;--dy:${dy}px;--flow-ms:${FLOW_MS}ms"></div>`,
      iconSize: [8, 8],
      iconAnchor: [4, 4],
    });
    // Koordinat flight konstan sepanjang hidupnya; hitung ikon sekali.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    const t = setTimeout(() => remove(flight.id), FLOW_MS + 60);
    return () => clearTimeout(t);
  }, [flight.id, remove]);

  return (
    <Marker
      position={[flight.fromLat, flight.fromLng]}
      icon={icon}
      interactive={false}
      keyboard={false}
    />
  );
}

/**
 * Lapisan partikel aliran data. Hanya me-render flight yang sedang aktif
 * (transien, dibatasi MAX_FLIGHTS), sehingga biayanya proporsional dengan lalu
 * lintas yang benar-benar terlihat — bukan jumlah kapal di peta.
 */
function FlowLayerInner() {
  const flights = useFlowStore(useShallow((s) => s.flights));
  return (
    <>
      {flights.map((f) => (
        <FlowDot key={f.id} flight={f} />
      ))}
    </>
  );
}

export const FlowLayer = memo(FlowLayerInner);
