import { describe, expect, it } from 'vitest';

import { BASE_DELAY_MS, JITTER_MS, MAX_DELAY_MS, reconnectDelayMs } from '@/lib/backoff';

describe('reconnectDelayMs', () => {
  const noJitter = () => 0;

  it('tumbuh eksponensial: 1s, 2s, 4s, 8s, 16s', () => {
    expect(reconnectDelayMs(0, noJitter)).toBe(1_000);
    expect(reconnectDelayMs(1, noJitter)).toBe(2_000);
    expect(reconnectDelayMs(2, noJitter)).toBe(4_000);
    expect(reconnectDelayMs(3, noJitter)).toBe(8_000);
    expect(reconnectDelayMs(4, noJitter)).toBe(16_000);
  });

  it('mentok di MAX_DELAY_MS berapa pun attempt-nya', () => {
    expect(reconnectDelayMs(5, noJitter)).toBe(MAX_DELAY_MS);
    expect(reconnectDelayMs(50, noJitter)).toBe(MAX_DELAY_MS);
    expect(reconnectDelayMs(1_000, noJitter)).toBe(MAX_DELAY_MS);
  });

  it('attempt negatif diperlakukan sebagai attempt 0', () => {
    expect(reconnectDelayMs(-3, noJitter)).toBe(BASE_DELAY_MS);
  });

  it('jitter berada dalam rentang 0..JITTER_MS', () => {
    const max = reconnectDelayMs(0, () => 0.999999);
    expect(max).toBeGreaterThanOrEqual(BASE_DELAY_MS);
    expect(max).toBeLessThan(BASE_DELAY_MS + JITTER_MS);
  });
});
