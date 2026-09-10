'use client';

import 'leaflet/dist/leaflet.css';

import { MapContainer, TileLayer, ZoomControl } from 'react-leaflet';
import { useShallow } from 'zustand/react/shallow';

import { EdgeMarker } from '@/components/map/EdgeMarker';
import { FlowLayer } from '@/components/map/FlowLayer';
import {
  DEFAULT_CENTER,
  DEFAULT_ZOOM,
  MAX_ZOOM,
  MIN_ZOOM,
  TILE_URL,
} from '@/components/map/mapConstants';
import { AutoFitOnce, FocusFlyer } from '@/components/map/MapControllers';
import { RouteLine } from '@/components/map/RouteLine';
import { ShipMarker } from '@/components/map/ShipMarker';
import { useTopologyStore } from '@/stores/topologyStore';

/**
 * Layer daftar kapal: hanya ber-subscribe ke DAFTAR ID (useShallow), bukan isi
 * node — penambahan/penghapusan kapal me-render layer ini, tetapi pergerakan
 * kapal hanya me-render ShipMarker miliknya sendiri (SRS §3B).
 */
function ShipMarkersLayer() {
  const ids = useTopologyStore(useShallow((s) => Object.keys(s.nodes)));
  return (
    <>
      {ids.map((id) => (
        <ShipMarker key={id} nodeId={id} />
      ))}
    </>
  );
}

function RouteLinesLayer() {
  const ids = useTopologyStore(useShallow((s) => Object.keys(s.nodes)));
  return (
    <>
      {ids.map((id) => (
        <RouteLine key={id} nodeId={id} />
      ))}
    </>
  );
}

function EdgeMarkersLayer() {
  const codes = useTopologyStore(useShallow((s) => Object.keys(s.edges)));
  return (
    <>
      {codes.map((code) => (
        <EdgeMarker key={code} edgeCode={code} />
      ))}
    </>
  );
}

/**
 * Kanvas peta fullscreen (PRD §3.1). Komponen ini WAJIB dimuat lewat
 * next/dynamic dengan ssr:false dari halaman pemanggil — Leaflet menyentuh
 * objek `window` saat modul dievaluasi sehingga crash di SSR (SRS §7.1).
 *
 * preferCanvas: seluruh CircleMarker & Polyline digambar pada satu elemen
 * <canvas> alih-alih ribuan node SVG — pilihan sadar untuk 50+ kapal yang
 * bergerak simultan; SVG lebih tajam saat zoom ekstrem tetapi biaya
 * reflow-nya per elemen, bukan per frame.
 */
export default function MapViewer() {
  return (
    <MapContainer
      center={DEFAULT_CENTER}
      zoom={DEFAULT_ZOOM}
      minZoom={MIN_ZOOM}
      maxZoom={MAX_ZOOM}
      preferCanvas
      zoomControl={false}
      // Kontrol atribusi ("Leaflet | © OpenStreetMap © CARTO") disembunyikan atas
      // permintaan produk agar sudut kanan-bawah bersih; kredit basemap tetap
      // dicantumkan di TILE_ATTRIBUTION (mapConstants) untuk pemenuhan lisensi.
      attributionControl={false}
      className="h-full w-full"
    >
      <TileLayer url={TILE_URL} maxZoom={MAX_ZOOM} />
      <ZoomControl position="bottomleft" />
      <EdgeMarkersLayer />
      <RouteLinesLayer />
      <FlowLayer />
      <ShipMarkersLayer />
      <AutoFitOnce />
      <FocusFlyer />
    </MapContainer>
  );
}
