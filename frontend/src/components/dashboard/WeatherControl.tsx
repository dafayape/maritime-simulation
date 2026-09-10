'use client';

import { useEffect, useRef, useState } from 'react';

import { GlassCard } from '@/components/ui/GlassCard';
import { ApiError, updateWeather } from '@/lib/api';
import { weatherLabel } from '@/lib/format';
import { MAX_WEATHER_SEVERITY, MIN_WEATHER_SEVERITY } from '@/lib/lora';
import { useSimulationStore } from '@/stores/simulationStore';

/**
 * Slider injeksi anomali cuaca (PRD "Badai buatan", SRS §4C).
 *
 * PUT /weather hanya ditembak saat user MELEPAS slider (commit), bukan pada
 * setiap piksel pergeseran — REST transaksional tidak boleh dibanjiri, dan
 * nilai final yang berlaku tetap datang balik dari backend lewat event
 * env:sync_params (satu sumber kebenaran).
 */
export function WeatherControl() {
  const sessionId = useSimulationStore((s) => s.sessionId);
  const active = useSimulationStore((s) => s.session?.is_active ?? false);
  const serverSeverity = useSimulationStore(
    (s) => s.envParams?.weather_severity ?? s.session?.weather_severity ?? 1.0,
  );

  const [value, setValue] = useState(serverSeverity);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dragging = useRef(false);

  // Ikuti nilai server (mis. diubah dari tab lain) selama tidak sedang digeser.
  useEffect(() => {
    if (!dragging.current) setValue(serverSeverity);
  }, [serverSeverity]);

  const commit = async () => {
    dragging.current = false;
    if (!sessionId || !active || value === serverSeverity) return;
    setBusy(true);
    setError(null);
    try {
      await updateWeather(sessionId, Number(value.toFixed(1)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'gagal memperbarui cuaca');
      setValue(serverSeverity);
    } finally {
      setBusy(false);
    }
  };

  return (
    <GlassCard
      title="Anomali Cuaca"
      action={
        <span className="text-xs font-semibold text-sky-300">
          {value.toFixed(1)} · {weatherLabel(value)}
        </span>
      }
    >
      <input
        type="range"
        min={MIN_WEATHER_SEVERITY}
        max={MAX_WEATHER_SEVERITY}
        step={0.1}
        value={value}
        disabled={!active || busy}
        onChange={(e) => {
          dragging.current = true;
          setValue(Number(e.target.value));
        }}
        onPointerUp={commit}
        onKeyUp={(e) => {
          if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') void commit();
        }}
        className="w-full accent-sky-400 disabled:opacity-40"
        aria-label="Tingkat keparahan cuaca"
      />
      <div className="mt-1 flex justify-between text-[10px] text-slate-400">
        <span>0.0 tenang</span>
        <span>1.0 normal</span>
        <span>2.0 badai</span>
      </div>
      {busy && <p className="mt-1 text-[11px] text-slate-400">Mengirim ke backend…</p>}
      {error && <p className="mt-1 text-[11px] text-rose-300">{error}</p>}
    </GlassCard>
  );
}
