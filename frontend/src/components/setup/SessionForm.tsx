'use client';

import { useRouter } from 'next/navigation';
import { useState } from 'react';

import { SchemaBuilder } from '@/components/setup/SchemaBuilder';
import { ApiError, createSimulation, setSchema } from '@/lib/api';
import { weatherLabel } from '@/lib/format';
import {
  ackTimeoutMs,
  MAX_SPREADING_FACTOR,
  MAX_TX_POWER_DBM,
  MAX_WEATHER_SEVERITY,
  MIN_SPREADING_FACTOR,
  MIN_TX_POWER_DBM,
  MIN_WEATHER_SEVERITY,
  maxPayloadBytes,
  maxRangeKm,
} from '@/lib/lora';
import type { SchemaField } from '@/types/backend';

/**
 * Form "Create Session" (SRS §4A): POST /api/v1/simulations, lalu bila skema
 * diisi POST /schema, kemudian redirect ke /dashboard?session=<uuid>.
 */
export function SessionForm() {
  const router = useRouter();
  const [name, setName] = useState('');
  const [sf, setSf] = useState(7);
  const [tx, setTx] = useState(20);
  const [weather, setWeather] = useState(1.0);
  const [fields, setFields] = useState<SchemaField[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const created = await createSimulation({
        session_name: name.trim(),
        spreading_factor: sf,
        tx_power_dbm: tx,
        weather_severity: Number(weather.toFixed(1)),
      });
      // Skema diinjeksi setelah sesi lahir; kegagalan skema tidak membatalkan
      // sesi (bisa diinjeksi ulang dari dasbor backend mana pun), tapi tetap
      // dilaporkan agar user sadar kapal belum menerima struktur payload.
      if (fields.length > 0) {
        try {
          await setSchema(created.session_id, fields);
        } catch (schemaErr) {
          const msg =
            schemaErr instanceof ApiError ? schemaErr.message : 'gagal menyimpan skema';
          window.alert(`Sesi dibuat, tetapi skema ditolak backend: ${msg}`);
        }
      }
      router.push(`/dashboard?session=${created.session_id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal membuat sesi — backend tidak terjangkau');
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-5">
      <label className="block text-xs text-slate-400">
        Nama Sesi
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          maxLength={150}
          placeholder="cth: Uji armada Pelabuhan Ratu — musim barat"
          className="mt-1 w-full rounded-md border border-white/10 bg-slate-800 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-500"
        />
      </label>

      <div>
        <div className="flex items-baseline justify-between text-xs text-slate-400">
          <span>Spreading Factor</span>
          <span className="font-semibold text-sky-300">
            SF{sf} · payload ≤{maxPayloadBytes(sf)} B · ACK {ackTimeoutMs(sf)} ms
          </span>
        </div>
        <input
          type="range"
          min={MIN_SPREADING_FACTOR}
          max={MAX_SPREADING_FACTOR}
          step={1}
          value={sf}
          onChange={(e) => setSf(Number(e.target.value))}
          className="mt-1 w-full accent-sky-400"
          aria-label="Spreading Factor"
        />
        <div className="flex justify-between text-[10px] text-slate-400">
          <span>SF7 — cepat, payload besar</span>
          <span>SF12 — jauh, payload 51 B</span>
        </div>
      </div>

      <div>
        <div className="flex items-baseline justify-between text-xs text-slate-400">
          <span>TX Power</span>
          <span className="font-semibold text-sky-300">
            {tx} dBm · jangkauan ±{maxRangeKm(tx).toFixed(1)} km
          </span>
        </div>
        <input
          type="range"
          min={MIN_TX_POWER_DBM}
          max={MAX_TX_POWER_DBM}
          step={1}
          value={tx}
          onChange={(e) => setTx(Number(e.target.value))}
          className="mt-1 w-full accent-sky-400"
          aria-label="TX Power dBm"
        />
      </div>

      <div>
        <div className="flex items-baseline justify-between text-xs text-slate-400">
          <span>Cuaca Awal</span>
          <span className="font-semibold text-sky-300">
            {weather.toFixed(1)} · {weatherLabel(weather)}
          </span>
        </div>
        <input
          type="range"
          min={MIN_WEATHER_SEVERITY}
          max={MAX_WEATHER_SEVERITY}
          step={0.1}
          value={weather}
          onChange={(e) => setWeather(Number(e.target.value))}
          className="mt-1 w-full accent-sky-400"
          aria-label="Weather severity awal"
        />
      </div>

      <SchemaBuilder fields={fields} onChange={setFields} payloadLimit={maxPayloadBytes(sf)} />

      {error && <p className="text-xs text-rose-300">{error}</p>}

      <button
        type="submit"
        disabled={busy || name.trim() === ''}
        className="w-full rounded-md bg-sky-600 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-sky-500 disabled:cursor-not-allowed disabled:opacity-40"
      >
        {busy ? 'Membuat sesi…' : 'Mulai Simulasi → Buka Dasbor'}
      </button>
    </form>
  );
}
