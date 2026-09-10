'use client';

import { useCallback, useEffect, useState } from 'react';

import { RefreshIcon } from '@/components/ui/icons';
import { backendHealthy } from '@/lib/api';
import { apiBaseUrl } from '@/lib/config';

type Status = 'checking' | 'up' | 'down';

/** Cek kesehatan backend sekali saat halaman dibuka + tombol cek ulang manual. */
export function BackendStatus() {
  const [status, setStatus] = useState<Status>('checking');

  const check = useCallback(() => {
    setStatus('checking');
    backendHealthy().then((ok) => setStatus(ok ? 'up' : 'down'));
  }, []);

  useEffect(check, [check]);

  const dot =
    status === 'up' ? 'bg-emerald-400' : status === 'down' ? 'bg-rose-400' : 'bg-slate-500 animate-pulse';
  const label =
    status === 'up'
      ? 'Backend terhubung'
      : status === 'down'
        ? 'Backend tidak terjangkau'
        : 'Memeriksa backend…';

  return (
    <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-slate-900/70 px-3 py-1.5 text-xs text-slate-300">
      <span className={`h-2 w-2 rounded-full ${dot}`} />
      {label}
      <span className="font-mono text-[10px] text-slate-400">{apiBaseUrl()}</span>
      <button
        type="button"
        onClick={check}
        disabled={status === 'checking'}
        aria-label="Periksa ulang koneksi backend"
        title="Periksa ulang koneksi backend"
        className="inline-grid h-6 w-6 place-items-center rounded-full text-sky-300 transition hover:bg-sky-500/15 hover:text-sky-200 disabled:opacity-50"
      >
        <RefreshIcon size={13} className={status === 'checking' ? 'animate-spin' : undefined} />
      </button>
    </div>
  );
}
