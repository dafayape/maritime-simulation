/**
 * MetricStore (SRS §2C): agregasi statistik jaringan di sisi browser.
 *
 * Nama counter mengikuti persis kunci Redis backend (service/stats.go).
 * `hydrate` menimpa nilai absolut dari REST /stats (saat connect/reconnect),
 * lalu setiap packet:event menaikkan counter kanonik yang sama — sehingga
 * angka dasbor tetap sinkron dengan server, bukan hitungan lokal semata.
 */

import { create } from 'zustand';

import { DROP_REASON_TO_STAT, STAT_KEYS } from '@/lib/lora';
import type { LiveStats, PacketEvent } from '@/types/backend';

export interface MetricState {
  counters: LiveStats;
  hydrate(stats: LiveStats | null): void;
  applyPacketEvent(ev: PacketEvent): void;
  reset(): void;
}

// Pemetaan tipe packet:event -> counter kanonik. Catatan penting: backend
// TIDAK menyiarkan packet:event bertipe "transmit" (konstanta PktTransmit ada
// tapi tak pernah dipakai) — jadi `transmit_total` di sini praktis hanya naik
// lewat hydrate() dari REST /stats, bukan dari stream WS. Entri `transmit`
// dipertahankan sebagai pemetaan defensif yang tetap benar bila suatu saat
// backend mulai menyiarkannya. Penyegaran real-nya ditangani polling /stats
// di useMonitorSocket (lihat STATS_POLL_MS).
const EVENT_TYPE_TO_STAT: Record<string, string> = {
  transmit: STAT_KEYS.transmitTotal,
  forward: STAT_KEYS.forwardedToNode,
  deliver: STAT_KEYS.deliveredToEdge,
  duplicate: STAT_KEYS.duplicatesFiltered,
  ack: STAT_KEYS.acksRelayed,
  ack_drop: STAT_KEYS.acksDropped,
};

export function statKeyForPacketEvent(ev: PacketEvent): string | null {
  if (ev.type === 'drop') {
    return DROP_REASON_TO_STAT[ev.reason ?? ''] ?? STAT_KEYS.droppedInvalid;
  }
  return EVENT_TYPE_TO_STAT[ev.type] ?? null;
}

export const useMetricStore = create<MetricState>()((set) => ({
  counters: {},

  hydrate: (stats) => {
    if (!stats) return;
    set({ counters: { ...stats } });
  },

  applyPacketEvent: (ev) =>
    set((state) => {
      const key = statKeyForPacketEvent(ev);
      if (!key) return state;
      return {
        counters: { ...state.counters, [key]: (state.counters[key] ?? 0) + 1 },
      };
    }),

  reset: () => set({ counters: {} }),
}));
