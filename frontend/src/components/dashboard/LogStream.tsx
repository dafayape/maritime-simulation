'use client';

import clsx from 'clsx';
import { useEffect, useRef } from 'react';

import { GlassCard } from '@/components/ui/GlassCard';
import { useLogStore, type LogEntry } from '@/stores/logStore';

function rowTone(entry: LogEntry): string {
  if (entry.kind === 'system') return 'text-slate-400 italic';
  switch (entry.packetType) {
    case 'deliver':
      return 'text-emerald-300';
    case 'drop':
    case 'ack_drop':
      return 'text-rose-300';
    case 'duplicate':
      return 'text-amber-300';
    case 'forward':
      return 'text-sky-300';
    case 'ack':
      return 'text-teal-300';
    default:
      return 'text-slate-300';
  }
}

/**
 * Log stream bergulir (SRS §6.2): auto-scroll hanya saat user memang berada
 * di dasar panel — begitu user menggulir ke atas untuk membaca riwayat,
 * baris baru tidak merebut posisi scroll.
 *
 * Jejak audit tidak dibatasi 200 baris: seluruh transmisi sesi dapat digulir
 * dari awal ke akhir. Agar ribuan baris tetap mulus, tiap baris memakai
 * `content-visibility: auto` (kelas `.log-row`) sehingga browser melewati
 * render baris di luar viewport — virtualisasi tanpa dependensi tambahan.
 */
export function LogStream() {
  const entries = useLogStore((s) => s.entries);
  const boxRef = useRef<HTMLDivElement | null>(null);
  const atBottomRef = useRef(true);

  useEffect(() => {
    const box = boxRef.current;
    if (box && atBottomRef.current) {
      box.scrollTop = box.scrollHeight;
    }
  }, [entries]);

  return (
    <GlassCard
      title="Log Transmisi"
      action={
        <span className="text-[10px] tabular-nums text-slate-400">
          {entries.length.toLocaleString('id-ID')} baris
        </span>
      }
    >
      <div
        ref={boxRef}
        onScroll={(e) => {
          const el = e.currentTarget;
          atBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
        }}
        className="thin-scroll h-52 space-y-0.5 overflow-y-auto overscroll-contain font-mono text-[11px] leading-4"
      >
        {entries.length === 0 && (
          <p className="text-slate-400">Menunggu lalu lintas jaringan…</p>
        )}
        {entries.map((entry) => (
          <p key={entry.seq} className={clsx('log-row break-words', rowTone(entry))}>
            <span className="text-slate-500">{entry.at}</span> {entry.text}
          </p>
        ))}
      </div>
    </GlassCard>
  );
}
