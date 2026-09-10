/**
 * FlowStore: antrean transien "partikel data" yang beranimasi mengalir dari
 * satu node ke parent-nya setiap kali sebuah paket berhasil di-relay (forward)
 * atau mendarat di Syahbandar (deliver). Murni lapisan visual — tidak
 * memengaruhi topologi/metrik, hanya membuat monitoring terasa hidup.
 *
 * Bukan ring buffer riwayat: tiap flight berumur pendek (~1 s) lalu digusur
 * oleh komponennya sendiri. Concurrency dibatasi MAX_FLIGHTS supaya badai
 * ratusan event/detik tidak menumbuhkan ratusan node DOM beranimasi — flight
 * terlama digusur saat penuh sehingga yang terlihat selalu yang terbaru.
 */

import { create } from 'zustand';

export type FlowKind = 'forward' | 'deliver';

export interface Flight {
  id: number;
  fromLat: number;
  fromLng: number;
  toLat: number;
  toLng: number;
  kind: FlowKind;
}

/** Batas flight beranimasi serentak — cukup ramai tanpa membebani DOM. */
export const MAX_FLIGHTS = 26;

export interface FlowState {
  flights: Flight[];
  spawn(f: Omit<Flight, 'id'>): void;
  remove(id: number): void;
  reset(): void;
}

let seq = 0;

export const useFlowStore = create<FlowState>()((set) => ({
  flights: [],

  spawn: (f) =>
    set((state) => {
      const next =
        state.flights.length >= MAX_FLIGHTS ? state.flights.slice(1) : state.flights.slice();
      next.push({ ...f, id: ++seq });
      return { flights: next };
    }),

  remove: (id) =>
    set((state) => ({ flights: state.flights.filter((f) => f.id !== id) })),

  reset: () => set({ flights: [] }),
}));
