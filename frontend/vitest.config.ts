import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

// Unit tests only cover pure logic (stores, event routing, LoRa math,
// backoff) so the plain "node" environment is enough — no jsdom needed.
export default defineConfig({
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
