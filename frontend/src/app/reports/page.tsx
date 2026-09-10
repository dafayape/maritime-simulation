'use client';

import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useCallback, useEffect, useState } from 'react';

import {
  ArrowLeftIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  RefreshIcon,
} from '@/components/ui/icons';
import { GlassCard } from '@/components/ui/GlassCard';
import { ApiError, getSimulation, listSimulations, listTelemetry } from '@/lib/api';
import { shortId, timeOnly } from '@/lib/format';
import { STAT_KEYS, totalDropped } from '@/lib/lora';
import type { Session, TelemetryLog } from '@/types/backend';

const PAGE_SIZE = 50;

/** Kartu ringkasan statistik sesi (live untuk sesi aktif, snapshot untuk sesi usai). */
function StatsSummary({ stats }: { stats: Record<string, number> }) {
  const sent = stats[STAT_KEYS.transmitTotal] ?? 0;
  const delivered = stats[STAT_KEYS.deliveredToEdge] ?? 0;
  const dropped = totalDropped(stats);
  const items: Array<[string, string]> = [
    ['Frame dikirim', sent.toLocaleString('id-ID')],
    ['Sampai Syahbandar', delivered.toLocaleString('id-ID')],
    ['Drop total', dropped.toLocaleString('id-ID')],
    ['Loss rate', sent > 0 ? `${((dropped / sent) * 100).toFixed(1)}%` : '—'],
    ['Retry tersaring', (stats[STAT_KEYS.duplicatesFiltered] ?? 0).toLocaleString('id-ID')],
    ['ACK sukses', (stats[STAT_KEYS.acksRelayed] ?? 0).toLocaleString('id-ID')],
  ];
  return (
    <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
      {items.map(([label, value]) => (
        <div key={label} className="rounded-lg bg-slate-800/60 px-3 py-2">
          <p className="text-base font-semibold">{value}</p>
          <p className="text-[10px] uppercase tracking-wide text-slate-400">{label}</p>
        </div>
      ))}
    </div>
  );
}

/**
 * Kode edge yang mengantarkan paket ini + status master-data-nya HARI INI:
 * hijau (edge masih terdaftar) atau merah (edge itu sudah dihapus dari
 * master data, tapi nama pengantarnya tetap dipertahankan dari snapshot
 * `edge_code_snapshot` — bukan lagi ditampilkan sebagai "—" yang mengaburkan
 * riwayat kirim).
 */
function EdgeBadge({ code, active }: { code?: string; active: boolean }) {
  if (!code) return <span className="font-mono text-slate-500">—</span>;
  return (
    <span
      className={
        'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 font-mono text-[11px] ' +
        (active
          ? 'bg-emerald-500/15 text-emerald-300'
          : 'bg-rose-500/15 text-rose-300')
      }
      title={active ? 'Edge masih terdaftar di master data' : 'Edge ini sudah dihapus dari master data'}
    >
      <span
        className={`h-1.5 w-1.5 shrink-0 rounded-full ${active ? 'bg-emerald-400' : 'bg-rose-400'}`}
      />
      {code}
    </span>
  );
}

function PayloadChips({ payload }: { payload: Record<string, unknown> | null }) {
  const entries = Object.entries(payload ?? {});
  if (entries.length === 0) return <span className="text-slate-400">—</span>;
  return (
    <span className="flex flex-wrap gap-1">
      {entries.map(([k, v]) => (
        <span key={k} className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px]">
          {k}: <span className="text-emerald-300">{String(v)}</span>
        </span>
      ))}
    </span>
  );
}

function ReportsInner() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sessionId = searchParams.get('session');

  const [sessions, setSessions] = useState<Session[]>([]);
  const [session, setSession] = useState<Session | null>(null);
  const [stats, setStats] = useState<Record<string, number>>({});
  const [rows, setRows] = useState<TelemetryLog[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Daftar sesi untuk pemilih di bagian atas halaman.
  useEffect(() => {
    listSimulations(50)
      .then((res) => setSessions(res ?? []))
      .catch(() => setSessions([]));
  }, []);

  const load = useCallback(
    async (id: string, pageOffset: number) => {
      setLoading(true);
      setError(null);
      try {
        const [detail, telemetry] = await Promise.all([
          getSimulation(id),
          listTelemetry(id, PAGE_SIZE, pageOffset),
        ]);
        setSession(detail.session);
        // Sesi aktif membaca counter Redis live; sesi selesai memakai
        // stats_snapshot yang dibekukan backend saat sesi dihentikan.
        setStats(
          detail.session.is_active
            ? (detail.live_stats ?? {})
            : (detail.session.stats_snapshot ?? detail.live_stats ?? {}),
        );
        setRows(telemetry.items ?? []);
        setTotal(telemetry.total);
        setOffset(pageOffset);
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'gagal memuat laporan');
      } finally {
        setLoading(false);
      }
    },
    [],
  );

  useEffect(() => {
    if (sessionId) void load(sessionId, 0);
  }, [sessionId, load]);

  return (
    <main className="min-h-dvh px-4 py-10">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-5">
        <header className="flex flex-wrap items-end justify-between gap-3">
          <div>
            <Link
              href="/"
              className="inline-flex items-center gap-1 text-xs text-slate-400 transition hover:text-slate-200"
            >
              <ArrowLeftIcon size={13} /> Beranda
            </Link>
            <h1 className="mt-1 text-2xl font-bold tracking-tight">Laporan Historis</h1>
            <p className="mt-1 text-sm text-slate-400">
              Rekam jejak paket yang berhasil mendarat di Syahbandar (SRS: Historical Reports).
            </p>
          </div>
          <label className="text-xs text-slate-400">
            Pilih sesi
            <select
              value={sessionId ?? ''}
              onChange={(e) => router.replace(`/reports?session=${e.target.value}`)}
              className="ml-2 rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 text-xs text-slate-100"
            >
              <option value="" disabled>
                — pilih —
              </option>
              {sessions.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.session_name} ({shortId(s.id)}){s.is_active ? ' · aktif' : ''}
                </option>
              ))}
            </select>
          </label>
        </header>

        {!sessionId && (
          <GlassCard>
            <p className="text-sm text-slate-400">
              Pilih sesi pada menu di atas untuk melihat statistik dan telemetrinya.
            </p>
          </GlassCard>
        )}

        {error && (
          <GlassCard>
            <p className="text-sm text-rose-300">{error}</p>
          </GlassCard>
        )}

        {session && (
          <>
            <GlassCard
              title={`Statistik — ${session.session_name}`}
              action={
                session.is_active ? (
                  <Link
                    href={`/dashboard?session=${session.id}`}
                    className="text-[11px] text-emerald-300 hover:underline"
                  >
                    sesi masih aktif → buka dasbor
                  </Link>
                ) : (
                  <span className="text-[11px] text-slate-400">
                    selesai {session.ended_at ? new Date(session.ended_at).toLocaleString('id-ID') : ''}
                  </span>
                )
              }
            >
              <StatsSummary stats={stats} />
            </GlassCard>

            <GlassCard
              title={`Telemetri Terkirim (${total.toLocaleString('id-ID')} baris)`}
              action={
                <button
                  type="button"
                  onClick={() => sessionId && void load(sessionId, offset)}
                  disabled={loading}
                  aria-label="Muat ulang telemetri"
                  title="Muat ulang telemetri"
                  className="inline-flex items-center gap-1.5 rounded-md border border-white/10 bg-slate-800 px-2.5 py-1 text-[11px] font-medium text-slate-200 transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <RefreshIcon size={13} className={loading ? 'animate-spin' : undefined} />
                  {loading ? 'Memuat…' : 'Muat ulang'}
                </button>
              }
            >
              <div className="overflow-x-auto">
                <table className="w-full min-w-[640px] text-left text-xs">
                  <thead>
                    <tr className="border-b border-white/10 text-[10px] uppercase tracking-wide text-slate-400">
                      <th className="py-2 pr-3">Tiba</th>
                      <th className="py-2 pr-3">Kapal Asal</th>
                      <th className="py-2 pr-3">Edge</th>
                      <th className="py-2 pr-3">Hop</th>
                      <th className="py-2 pr-3">Jalur</th>
                      <th className="py-2">Payload</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.length === 0 && (
                      <tr>
                        <td colSpan={6} className="py-4 text-center text-slate-400">
                          {loading ? 'Memuat…' : 'Belum ada paket yang mendarat.'}
                        </td>
                      </tr>
                    )}
                    {rows.map((row) => (
                      <tr key={row.id} className="border-b border-white/5 align-top">
                        <td className="py-2 pr-3 font-mono text-slate-400">
                          {timeOnly(row.arrived_at)}
                        </td>
                        <td className="py-2 pr-3 font-mono">{row.origin_node_id}</td>
                        <td className="py-2 pr-3">
                          <EdgeBadge code={row.edge_code} active={row.edge_active} />
                        </td>
                        <td className="py-2 pr-3">{row.hop_count}</td>
                        <td className="py-2 pr-3 font-mono text-[10px] text-slate-400">
                          {row.routing_path}
                        </td>
                        <td className="py-2">
                          <PayloadChips payload={row.decoded_payload} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="mt-3 flex items-center justify-between text-xs text-slate-400">
                <button
                  type="button"
                  disabled={offset === 0 || loading}
                  onClick={() => sessionId && void load(sessionId, Math.max(0, offset - PAGE_SIZE))}
                  className="inline-flex items-center gap-1 rounded-md bg-slate-800 px-3 py-1.5 transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <ChevronLeftIcon size={14} /> Sebelumnya
                </button>
                <span className="tabular-nums">
                  {total === 0 ? 0 : offset + 1}–{Math.min(offset + PAGE_SIZE, total)} dari {total}
                </span>
                <button
                  type="button"
                  disabled={offset + PAGE_SIZE >= total || loading}
                  onClick={() => sessionId && void load(sessionId, offset + PAGE_SIZE)}
                  className="inline-flex items-center gap-1 rounded-md bg-slate-800 px-3 py-1.5 transition hover:bg-slate-700 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  Berikutnya <ChevronRightIcon size={14} />
                </button>
              </div>
            </GlassCard>
          </>
        )}
      </div>
    </main>
  );
}

export default function ReportsPage() {
  return (
    <Suspense
      fallback={
        <main className="grid h-dvh place-items-center text-sm text-slate-400">
          Memuat laporan…
        </main>
      }
    >
      <ReportsInner />
    </Suspense>
  );
}
