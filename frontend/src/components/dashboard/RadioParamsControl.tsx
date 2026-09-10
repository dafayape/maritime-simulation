'use client';

import { useEffect, useState } from 'react';

import { GlassCard } from '@/components/ui/GlassCard';
import { ApiError, updateRadioParams } from '@/lib/api';
import {
  MAX_SPREADING_FACTOR,
  MAX_TX_POWER_DBM,
  MIN_SPREADING_FACTOR,
  MIN_TX_POWER_DBM,
  maxPayloadBytes,
  maxRangeKm,
} from '@/lib/lora';
import { useSimulationStore } from '@/stores/simulationStore';

const SF_OPTIONS = Array.from(
  { length: MAX_SPREADING_FACTOR - MIN_SPREADING_FACTOR + 1 },
  (_, i) => MIN_SPREADING_FACTOR + i,
);

/**
 * Pengubah parameter radio runtime (PRD: "mengubah batasan LoRa secara
 * dinamis"). PUT /params memicu backend menyiarkan env:sync_params ke semua
 * node + monitor, sehingga chip StatusBar ter-update tanpa aksi tambahan.
 */
export function RadioParamsControl() {
  const sessionId = useSimulationStore((s) => s.sessionId);
  const active = useSimulationStore((s) => s.session?.is_active ?? false);
  const currentSf = useSimulationStore(
    (s) => s.envParams?.sf ?? s.session?.spreading_factor ?? 7,
  );
  const currentTx = useSimulationStore(
    (s) => s.envParams?.tx_power_dbm ?? s.session?.tx_power_dbm ?? 20,
  );

  const [sf, setSf] = useState(currentSf);
  const [tx, setTx] = useState(currentTx);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setSf(currentSf), [currentSf]);
  useEffect(() => setTx(currentTx), [currentTx]);

  const dirty = sf !== currentSf || tx !== currentTx;

  const apply = async () => {
    if (!sessionId || !active || !dirty || busy) return;
    setBusy(true);
    setError(null);
    try {
      await updateRadioParams(sessionId, { spreading_factor: sf, tx_power_dbm: tx });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal memperbarui parameter');
      setSf(currentSf);
      setTx(currentTx);
    } finally {
      setBusy(false);
    }
  };

  return (
    <GlassCard title="Parameter Radio">
      <div className="grid grid-cols-2 gap-3">
        <label className="block text-xs text-slate-400">
          Spreading Factor
          <select
            value={sf}
            disabled={!active || busy}
            onChange={(e) => setSf(Number(e.target.value))}
            className="mt-1 w-full rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 text-sm text-slate-100 disabled:opacity-40"
          >
            {SF_OPTIONS.map((option) => (
              <option key={option} value={option}>
                SF{option} · ≤{maxPayloadBytes(option)} B
              </option>
            ))}
          </select>
        </label>
        <label className="block text-xs text-slate-400">
          TX Power (dBm)
          {/* no-spinner: sembunyikan tombol naik/turun bawaan input number
              (lihat globals.css) — nilai cukup diketik / lewat rentang valid. */}
          <input
            type="number"
            min={MIN_TX_POWER_DBM}
            max={MAX_TX_POWER_DBM}
            value={tx}
            disabled={!active || busy}
            onChange={(e) => setTx(Number(e.target.value))}
            className="no-spinner mt-1 w-full rounded-md border border-white/10 bg-slate-800 px-2 py-1.5 text-sm text-slate-100 disabled:opacity-40"
          />
          <span className="mt-0.5 block text-[10px] text-slate-400">
            ±{maxRangeKm(Number.isFinite(tx) ? tx : currentTx).toFixed(1)} km jangkauan
          </span>
        </label>
      </div>
      <button
        type="button"
        onClick={apply}
        disabled={!active || !dirty || busy}
        className="mt-2 w-full rounded-md bg-sky-600/90 px-3 py-1.5 text-xs font-medium text-white transition hover:bg-sky-600 disabled:cursor-not-allowed disabled:opacity-40"
      >
        {busy ? 'Menerapkan…' : 'Terapkan Parameter'}
      </button>
      {error && <p className="mt-1.5 text-[11px] text-rose-300">{error}</p>}
    </GlassCard>
  );
}
