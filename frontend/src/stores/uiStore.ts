/**
 * UiStore: state antarmuka lintas-komponen yang tidak berasal dari backend.
 * Saat ini hanya "focus node" — klik item di panel daftar kapal membuat
 * kamera peta terbang (flyTo) ke marker kapal tersebut.
 */

import { create } from 'zustand';

export interface UiState {
  /** Naik setiap kali fokus diminta, agar klik node yang sama tetap memicu flyTo. */
  focusRequestId: number;
  focusNodeId: string | null;
  requestFocus(nodeId: string): void;
}

export const useUiStore = create<UiState>()((set) => ({
  focusRequestId: 0,
  focusNodeId: null,
  requestFocus: (nodeId) =>
    set((state) => ({ focusNodeId: nodeId, focusRequestId: state.focusRequestId + 1 })),
}));
