/**
 * Exponential backoff untuk auto-reconnect WebSocket (SRS §5A).
 *
 * Jadwal: 1s, 2s, 4s, 8s, 16s, lalu mentok di 30s — plus jitter acak 0..250ms
 * agar puluhan tab dasbor yang terputus bersamaan (backend restart) tidak
 * menyerbu server pada detik yang sama (thundering herd).
 */

export const BASE_DELAY_MS = 1_000;
export const MAX_DELAY_MS = 30_000;
export const JITTER_MS = 250;

/**
 * Delay milidetik sebelum percobaan reconnect ke-`attempt` (mulai dari 0).
 * `random` bisa diinjeksi supaya deterministik saat unit test.
 */
export function reconnectDelayMs(attempt: number, random: () => number = Math.random): number {
  const exp = Math.min(Math.max(attempt, 0), 30); // cegah overflow 2^attempt
  const base = Math.min(BASE_DELAY_MS * 2 ** exp, MAX_DELAY_MS);
  return base + Math.floor(random() * JITTER_MS);
}
