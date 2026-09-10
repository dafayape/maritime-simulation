'use client';

import clsx from 'clsx';
import Link from 'next/link';
import { useCallback, useEffect, useState } from 'react';

import { TrashIcon } from '@/components/ui/icons';
import { ApiError, deleteSession, listSimulations } from '@/lib/api';
import { shortId } from '@/lib/format';
import type { Session } from '@/types/backend';

/**
 * Daftar sesi yang sudah ada: sesi aktif bisa langsung dilanjutkan ke
 * dasbor (dasbor bersifat observer — berapa pun boleh menonton), sesi
 * selesai diarahkan ke halaman laporan dan dapat dihapus riwayatnya
 * (tombol tong sampah di sebelah kanan tombol Laporan/Buka Dasbor).
 */
export function SessionList() {
  const [sessions, setSessions] = useState<Session[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const refresh = useCallback(() => {
    listSimulations(20)
      .then((res) => setSessions(res ?? []))
      .catch(() => setError('backend tidak terjangkau'));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleDelete = async (s: Session) => {
    const ok = window.confirm(
      `Hapus riwayat sesi "${s.session_name}"?\n\n` +
        'Seluruh skema dan telemetri yang terkait ikut terhapus permanen. ' +
        'Tindakan ini tidak dapat dibatalkan.',
    );
    if (!ok) return;
    setDeleting(s.id);
    setDeleteError(null);
    try {
      await deleteSession(s.id);
      refresh();
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : 'gagal menghapus riwayat sesi');
    } finally {
      setDeleting(null);
    }
  };

  if (error) return <p className="text-xs text-rose-300">{error}</p>;
  if (sessions === null) return <p className="text-xs text-slate-400">Memuat sesi…</p>;
  if (sessions.length === 0)
    return <p className="text-xs text-slate-400">Belum ada sesi tersimpan.</p>;

  return (
    <div className="space-y-1.5">
      {deleteError && <p className="text-xs text-rose-300">{deleteError}</p>}
      <ul className="space-y-1.5">
        {sessions.map((s) => (
          <li
            key={s.id}
            className="flex items-center gap-3 rounded-md border border-white/5 bg-slate-800/50 px-3 py-2"
          >
            <span
              className={`h-2 w-2 shrink-0 rounded-full ${s.is_active ? 'bg-emerald-400' : 'bg-slate-600'}`}
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium text-slate-200" title={s.session_name}>
                {s.session_name}
              </p>
              <p className="text-[10px] text-slate-400">
                {shortId(s.id)} · SF{s.spreading_factor} · {s.tx_power_dbm} dBm ·{' '}
                {new Date(s.created_at).toLocaleString('id-ID')}
              </p>
            </div>
            {s.is_active ? (
              <Link
                href={`/dashboard?session=${s.id}`}
                className="shrink-0 rounded-md bg-emerald-600/80 px-2.5 py-1 text-[11px] font-medium text-white hover:bg-emerald-600"
              >
                Buka Dasbor
              </Link>
            ) : (
              <Link
                href={`/reports?session=${s.id}`}
                className="shrink-0 rounded-md bg-slate-700 px-2.5 py-1 text-[11px] font-medium text-slate-200 hover:bg-slate-600"
              >
                Laporan
              </Link>
            )}
            <button
              type="button"
              onClick={() => handleDelete(s)}
              disabled={s.is_active || deleting === s.id}
              aria-label={`Hapus riwayat sesi ${s.session_name}`}
              title={
                s.is_active
                  ? 'Hentikan sesi terlebih dahulu untuk menghapus riwayatnya'
                  : 'Hapus riwayat sesi'
              }
              className="inline-grid h-7 w-7 shrink-0 place-items-center rounded-md text-slate-400 transition hover:bg-rose-500/15 hover:text-rose-300 focus-visible:text-rose-300 disabled:opacity-30 disabled:hover:bg-transparent disabled:hover:text-slate-400"
            >
              <TrashIcon size={14} className={clsx(deleting === s.id && 'animate-pulse')} />
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
