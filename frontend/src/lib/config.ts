/**
 * Resolusi alamat backend.
 *
 * NEXT_PUBLIC_* di-inline oleh Next.js saat build, sehingga akses HARUS
 * berupa literal `process.env.NEXT_PUBLIC_X` (bukan akses dinamis).
 *
 * Konvensi deployment:
 *  - Dev lokal          : NEXT_PUBLIC_API_URL=http://localhost:8080
 *  - VPS di balik nginx : biarkan kosong -> dipakai origin browser sendiri,
 *    nginx yang meneruskan /api/v1 dan /ws ke container backend.
 */

const FALLBACK_API_URL = 'http://localhost:8080';

function stripTrailingSlash(url: string): string {
  return url.replace(/\/+$/, '');
}

/** Base URL REST API backend, tanpa trailing slash. */
export function apiBaseUrl(): string {
  const raw = (process.env.NEXT_PUBLIC_API_URL ?? '').trim();
  if (raw) return stripTrailingSlash(raw);
  if (typeof window !== 'undefined') return window.location.origin;
  return FALLBACK_API_URL;
}

/** Base URL WebSocket (ws:// atau wss://), diturunkan dari base API. */
export function wsBaseUrl(): string {
  const raw = (process.env.NEXT_PUBLIC_WS_URL ?? '').trim();
  if (raw) return stripTrailingSlash(raw);
  // http -> ws, https -> wss; skema lain dibiarkan (tidak valid untuk WS).
  return apiBaseUrl().replace(/^http/, 'ws');
}

/** URL lengkap kanal monitor dasbor untuk satu sesi. */
export function monitorSocketUrl(sessionId: string): string {
  return `${wsBaseUrl()}/ws/monitor?session_id=${encodeURIComponent(sessionId)}`;
}
