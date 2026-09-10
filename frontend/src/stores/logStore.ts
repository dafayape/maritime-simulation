/**
 * LogStore: buffer baris log untuk panel stream dasbor.
 *
 * Log adalah jejak audit penuh — seluruh transmisi dari awal sampai akhir sesi
 * harus dapat digulir kembali, jadi TIDAK dipangkas ke ratusan baris. Ribuan
 * baris ditanggung mulus oleh `content-visibility` pada baris (lihat LogStream
 * + globals.css) yang melewati render baris di luar layar. MAX_LOG_ENTRIES
 * tinggal batas pengaman ekstrem agar sesi yang berjalan berhari-hari tidak
 * menghabiskan memori tab tanpa batas; baris tertua baru digusur di ambang itu.
 */

import { create } from 'zustand';

import { describePacketEvent, timeOnly } from '@/lib/format';
import type { PacketEvent, PacketEventType } from '@/types/backend';

export const MAX_LOG_ENTRIES = 50_000;

export type LogLevel = 'info' | 'warn' | 'error';

export interface LogEntry {
  seq: number;
  at: string;
  kind: 'packet' | 'system';
  level: LogLevel;
  text: string;
  packetType?: PacketEventType;
}

export interface LogState {
  entries: LogEntry[];
  appendPacket(ev: PacketEvent): void;
  appendSystem(text: string, level?: LogLevel): void;
  reset(): void;
}

let seq = 0;

function push(entries: LogEntry[], entry: LogEntry): LogEntry[] {
  const next = entries.length >= MAX_LOG_ENTRIES ? entries.slice(1) : entries.slice();
  next.push(entry);
  return next;
}

function levelFor(type: PacketEventType): LogLevel {
  if (type === 'drop' || type === 'ack_drop') return 'error';
  if (type === 'duplicate') return 'warn';
  return 'info';
}

export const useLogStore = create<LogState>()((set) => ({
  entries: [],

  appendPacket: (ev) =>
    set((state) => ({
      entries: push(state.entries, {
        seq: ++seq,
        at: timeOnly(ev.at) || timeOnly(new Date().toISOString()),
        kind: 'packet',
        level: levelFor(ev.type),
        text: describePacketEvent(ev),
        packetType: ev.type,
      }),
    })),

  appendSystem: (text, level = 'info') =>
    set((state) => ({
      entries: push(state.entries, {
        seq: ++seq,
        at: timeOnly(new Date().toISOString()),
        kind: 'system',
        level,
        text,
      }),
    })),

  reset: () => set({ entries: [] }),
}));
