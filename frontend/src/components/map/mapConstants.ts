import type { LatLngTuple } from 'leaflet';

/** Pusat default: Pelabuhan Ratu — lokasi Virtual Edge EDGE-PRATU-01 yang
 *  di-seed migrasi backend, sehingga peta kosong pun sudah menghadap laut
 *  yang benar sebelum kapal pertama muncul. */
export const DEFAULT_CENTER: LatLngTuple = [-6.9875, 106.5504];
export const DEFAULT_ZOOM = 11;

/** Warna status marker kapal — 3 nilai inti dikunci SRS §3A; `offline`
 *  ditambahkan (abu-abu netral) agar kapal yang terputus terbaca "pergi/redup",
 *  bukan "error" seperti merah isolated. */
export const NODE_COLOR = {
  connected: '#10B981',
  retrying: '#F59E0B',
  isolated: '#EF4444',
  offline: '#64748B',
} as const;

/** Warna garis rute mesh (biru muda putus-putus, SRS §3A.2). */
export const ROUTE_COLOR = '#38BDF8';

/**
 * Basemap gelap CARTO (di atas data OpenStreetMap). Batas zoom dikunci agar
 * cache tile browser tidak membengkak (PRD §3.5 Map Tile Performance).
 */
export const TILE_URL = 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png';
export const TILE_ATTRIBUTION =
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/attributions">CARTO</a>';
export const MIN_ZOOM = 3;
export const MAX_ZOOM = 17;
