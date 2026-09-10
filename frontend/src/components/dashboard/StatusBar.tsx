'use client';

import clsx from 'clsx';
import Link from 'next/link';
import { useState } from 'react';

import { FileTextIcon, PlusIcon, StopIcon } from '@/components/ui/icons';
import { GlassCard } from '@/components/ui/GlassCard';
import { ApiError, stopSimulation } from '@/lib/api';
import { shortId, weatherLabel } from '@/lib/format';
import { ackTimeoutMs, maxPayloadBytes, maxRangeKm } from '@/lib/lora';
import { useSimulationStore, type WsStatus } from '@/stores/simulationStore';

const WS_BADGE: Record<WsStatus, { label: string; className: string }> = {
  idle: { label: 'Menyiapkan…', className: 'bg-slate-600/40 text-slate-300' },
  connecting: { label: 'Menghubungkan…', className: 'bg-sky-500/20 text-sky-300' },
  open: { label: 'Live', className: 'bg-emerald-500/20 text-emerald-300' },
  reconnecting: { label: 'Reconnect…', className: 'bg-amber-500/20 text-amber-300' },
  ended: { label: 'Sesi berakhir', className: 'bg-rose-500/20 text-rose-300' },
};

function Chip({ label, value }: { label: string; value: string }) {
  return (
    <span className="rounded-md bg-slate-800/80 px-2 py-1 text-[11px] text-slate-300">
      {label} <span className="font-semibold text-slate-100">{value}</span>
    </span>
  );
}

/**
 * Identitas sesi + parameter radio berjalan. Sumber utama chip adalah event
 * env:sync_params; sebelum event pertama tiba, nilainya disintesis dari
 * record sesi REST memakai mirror rumus LoRa (lib/lora) agar UI tidak kosong.
 */
export function StatusBar() {
  const session = useSimulationStore((s) => s.session);
  const envParams = useSimulationStore((s) => s.envParams);
  const wsStatus = useSimulationStore((s) => s.wsStatus);
  const [stopping, setStopping] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sf = envParams?.sf ?? session?.spreading_factor;
  const tx = envParams?.tx_power_dbm ?? session?.tx_power_dbm;
  const weather = envParams?.weather_severity ?? session?.weather_severity;
  const payload = envParams?.max_payload_bytes ?? (sf !== undefined ? maxPayloadBytes(sf) : undefined);
  const range = envParams?.max_range_km ?? (tx !== undefined ? maxRangeKm(tx) : undefined);
  const ackMs = envParams?.ack_timeout_ms ?? (sf !== undefined ? ackTimeoutMs(sf) : undefined);

  const badge = WS_BADGE[wsStatus];
  const active = session?.is_active ?? false;

  const onStop = async () => {
    if (!session || stopping) return;
    if (!window.confirm(`Hentikan sesi "${session.session_name}"?`)) return;
    setStopping(true);
    setError(null);
    try {
      await stopSimulation(session.id);
      // Backend menyiarkan session:ended ke semua socket; store akan
      // menandai sesi berakhir dari event tersebut.
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal menghentikan sesi');
    } finally {
      setStopping(false);
    }
  };

  return (
    <GlassCard>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="truncate text-base font-semibold" title={session?.session_name}>
            {session?.session_name ?? 'Memuat sesi…'}
          </h1>
          {session && (
            <p className="text-[11px] text-slate-400">
              ID <span className="font-mono">{shortId(session.id)}</span>
              {' · '}
              {active ? 'aktif' : 'selesai'}
            </p>
          )}
        </div>
        <span
          className={clsx(
            'flex shrink-0 items-center gap-1.5 rounded-full px-2.5 py-1 text-[11px] font-medium',
            badge.className,
          )}
        >
          <span
            className={clsx(
              'h-1.5 w-1.5 rounded-full bg-current',
              wsStatus === 'open' && 'animate-pulse',
            )}
          />
          {badge.label}
        </span>
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {sf !== undefined && <Chip label="SF" value={`${sf}`} />}
        {payload !== undefined && <Chip label="Payload" value={`≤${payload} B`} />}
        {tx !== undefined && <Chip label="TX" value={`${tx} dBm`} />}
        {range !== undefined && <Chip label="Jangkauan" value={`±${range.toFixed(1)} km`} />}
        {ackMs !== undefined && <Chip label="ACK" value={`${ackMs} ms`} />}
        {weather !== undefined && (
          <Chip label="Cuaca" value={`${weather.toFixed(1)} · ${weatherLabel(weather)}`} />
        )}
      </div>

      <div className="mt-3 flex items-center gap-2">
        <button
          type="button"
          onClick={onStop}
          disabled={!active || stopping}
          className="inline-flex items-center gap-1.5 rounded-md bg-rose-600/80 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-rose-600 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <StopIcon size={14} />
          {stopping ? 'Menghentikan…' : 'Hentikan Sesi'}
        </button>
        {session && (
          <Link
            href={`/reports?session=${session.id}`}
            className="inline-flex items-center gap-1.5 rounded-md bg-slate-700/80 px-3 py-1.5 text-xs font-medium text-slate-100 transition hover:bg-slate-700"
          >
            <FileTextIcon size={14} />
            Laporan
          </Link>
        )}
        <Link
          href="/setup"
          className="inline-flex items-center gap-1.5 rounded-md bg-slate-700/80 px-3 py-1.5 text-xs font-medium text-slate-100 transition hover:bg-slate-700"
        >
          <PlusIcon size={14} />
          Sesi Baru
        </Link>
      </div>
      {error && <p className="mt-2 text-xs text-rose-300">{error}</p>}
    </GlassCard>
  );
}
