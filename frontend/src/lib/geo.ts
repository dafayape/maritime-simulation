/**
 * Kalkulasi geometri ringan untuk kebutuhan RENDER saja (panah arah rute).
 * Perhitungan jarak/rute yang sesungguhnya adalah domain backend Go
 * (PRD Non-Goals) — file ini tidak boleh dipakai untuk logika routing.
 */

/** Initial great-circle bearing dari titik 1 ke titik 2, dalam derajat 0..360. */
export function bearingDeg(lat1: number, lng1: number, lat2: number, lng2: number): number {
  const toRad = Math.PI / 180;
  const phi1 = lat1 * toRad;
  const phi2 = lat2 * toRad;
  const dLng = (lng2 - lng1) * toRad;
  const y = Math.sin(dLng) * Math.cos(phi2);
  const x = Math.cos(phi1) * Math.sin(phi2) - Math.sin(phi1) * Math.cos(phi2) * Math.cos(dLng);
  return (Math.atan2(y, x) * (180 / Math.PI) + 360) % 360;
}

/**
 * Titik tengah aritmetika — cukup akurat untuk penempatan ikon panah pada
 * segmen rute LoRa (≤ ~20 km); bukan midpoint great-circle sejati.
 */
export function midpoint(
  lat1: number,
  lng1: number,
  lat2: number,
  lng2: number,
): [number, number] {
  return [(lat1 + lat2) / 2, (lng1 + lng2) / 2];
}
