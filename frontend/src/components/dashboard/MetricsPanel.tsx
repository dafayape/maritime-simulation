'use client';

import { useShallow } from 'zustand/react/shallow';

import { GlassCard } from '@/components/ui/GlassCard';
import { formatPercent } from '@/lib/format';
import { STAT_KEYS, totalDropped } from '@/lib/lora';
import { useMetricStore } from '@/stores/metricStore';
import { useTopologyStore } from '@/stores/topologyStore';

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="rounded-lg bg-slate-800/60 px-3 py-2">
      <p className={`text-lg font-semibold leading-tight tabular-nums ${tone ?? 'text-slate-100'}`}>
        {value}
      </p>
      <p className="text-[10px] uppercase tracking-wide text-slate-400">{label}</p>
    </div>
  );
}

/**
 * Metrik jaringan (SRS §2C + §6.2): counter kanonik backend + agregat
 * turunan (persentase kehilangan paket dihitung di sisi klien, PRD §3.3).
 */
export function MetricsPanel() {
  const counters = useMetricStore((s) => s.counters);
  // "Terisolasi" hanya menghitung kapal yang SEDANG online tapi tak punya rute
  // ke Edge — angka yang benar-benar bermakna operasional. Kapal offline tidak
  // dihitung terisolasi (mereka sudah pergi, bukan terdampar), jadi armada yang
  // selesai membaca 0 terisolasi + 0/N aktif secara konsisten, bukan "semua merah".
  const [totalNodes, onlineNodes, isolatedNodes] = useTopologyStore(
    useShallow((s) => {
      let online = 0;
      let isolated = 0;
      const all = Object.values(s.nodes);
      for (const n of all) {
        if (n.online) {
          online += 1;
          if (n.status === 'isolated') isolated += 1;
        }
      }
      return [all.length, online, isolated] as const;
    }),
  );

  const sent = counters[STAT_KEYS.transmitTotal] ?? 0;
  const delivered = counters[STAT_KEYS.deliveredToEdge] ?? 0;
  const forwarded = counters[STAT_KEYS.forwardedToNode] ?? 0;
  const dropped = totalDropped(counters);
  const duplicates = counters[STAT_KEYS.duplicatesFiltered] ?? 0;
  const acks = counters[STAT_KEYS.acksRelayed] ?? 0;
  const lossRate = sent > 0 ? dropped / sent : 0;

  return (
    <GlassCard title="Metrik Jaringan">
      <div className="grid grid-cols-3 gap-2">
        <Stat label="Kapal Aktif" value={`${onlineNodes}/${totalNodes}`} />
        <Stat
          label="Terisolasi"
          value={`${isolatedNodes}`}
          tone={isolatedNodes > 0 ? 'text-rose-300' : 'text-emerald-300'}
        />
        <Stat
          label="Loss Rate"
          value={formatPercent(lossRate)}
          tone={lossRate > 0.3 ? 'text-rose-300' : lossRate > 0.1 ? 'text-amber-300' : 'text-emerald-300'}
        />
        <Stat label="Frame Kirim" value={sent.toLocaleString('id-ID')} />
        <Stat label="Sampai Edge" value={delivered.toLocaleString('id-ID')} tone="text-emerald-300" />
        <Stat label="Drop" value={dropped.toLocaleString('id-ID')} tone="text-rose-300" />
        <Stat label="Relay Antar-Kapal" value={forwarded.toLocaleString('id-ID')} />
        <Stat label="Retry Tersaring" value={duplicates.toLocaleString('id-ID')} tone="text-amber-300" />
        <Stat label="ACK Sukses" value={acks.toLocaleString('id-ID')} />
      </div>
    </GlassCard>
  );
}
