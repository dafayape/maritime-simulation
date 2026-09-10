/**
 * SimulationStore (SRS §2B): identitas sesi aktif, parameter radio yang
 * sedang berlaku (dari event env:sync_params), skema payload, dan status
 * koneksi WebSocket dasbor.
 */

import { create } from 'zustand';

import type { EnvSyncParamsEvent, SchemaField, Session } from '@/types/backend';

export type WsStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'ended';

export interface ActiveSchema {
  fields: SchemaField[];
  estimatedBytes: number;
}

export interface SimulationState {
  sessionId: string | null;
  session: Session | null;
  /** Parameter lingkungan radio terkini — sumber: WS env:sync_params. */
  envParams: EnvSyncParamsEvent | null;
  schema: ActiveSchema | null;
  wsStatus: WsStatus;
  endedMessage: string | null;
  setSessionId(id: string | null): void;
  setSession(session: Session): void;
  setEnvParams(params: EnvSyncParamsEvent): void;
  setSchema(fields: SchemaField[] | null, estimatedBytes: number): void;
  setWsStatus(status: WsStatus): void;
  markEnded(message: string): void;
  reset(): void;
}

export const useSimulationStore = create<SimulationState>()((set) => ({
  sessionId: null,
  session: null,
  envParams: null,
  schema: null,
  wsStatus: 'idle',
  endedMessage: null,

  setSessionId: (id) =>
    set((state) =>
      state.sessionId === id
        ? state
        : {
            sessionId: id,
            session: null,
            envParams: null,
            schema: null,
            wsStatus: 'idle',
            endedMessage: null,
          },
    ),

  setSession: (session) => set({ session }),

  setEnvParams: (params) => set({ envParams: params }),

  setSchema: (fields, estimatedBytes) =>
    set({ schema: { fields: fields ?? [], estimatedBytes } }),

  setWsStatus: (wsStatus) => set({ wsStatus }),

  markEnded: (message) =>
    set((state) => ({
      wsStatus: 'ended',
      endedMessage: message,
      session: state.session ? { ...state.session, is_active: false } : state.session,
    })),

  reset: () =>
    set({
      sessionId: null,
      session: null,
      envParams: null,
      schema: null,
      wsStatus: 'idle',
      endedMessage: null,
    }),
}));
