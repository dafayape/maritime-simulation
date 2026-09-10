'use client';

import dynamic from 'next/dynamic';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import { Suspense, useEffect, useState } from 'react';

import { LogStream } from '@/components/dashboard/LogStream';
import { MetricsPanel } from '@/components/dashboard/MetricsPanel';
import { NodeListPanel } from '@/components/dashboard/NodeListPanel';
import { RadioParamsControl } from '@/components/dashboard/RadioParamsControl';
import { StatusBar } from '@/components/dashboard/StatusBar';
import { WeatherControl } from '@/components/dashboard/WeatherControl';
import { ApiError, getSchema, getSimulation, getStats, getTopology } from '@/lib/api';
import { useMonitorSocket } from '@/hooks/useMonitorSocket';
import { useFlowStore } from '@/stores/flowStore';
import { useLogStore } from '@/stores/logStore';
import { useMetricStore } from '@/stores/metricStore';
import { useSimulationStore } from '@/stores/simulationStore';
import { useTopologyStore } from '@/stores/topologyStore';

// Leaflet mengakses `window` saat modulnya dievaluasi — WAJIB dimuat dinamis
// tanpa SSR (SRS §7.1); shell HTML server hanya berisi placeholder gelap.
const MapViewer = dynamic(() => import('@/components/map/MapViewer'), {
  ssr: false,
  loading: () => (
    <div className="grid h-full w-full place-items-center bg-slate-950 text-sm text-slate-500">
      Memuat kanvas peta…
    </div>
  ),
});

function DashboardInner() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const sessionId = searchParams.get('session');

  const wsStatus = useSimulationStore((s) => s.wsStatus);
  const endedMessage = useSimulationStore((s) => s.endedMessage);
  const [pageError, setPageError] = useState<string | null>(null);

  // Tanpa ?session=<uuid> dasbor tidak punya konteks — kembali ke /setup.
  useEffect(() => {
    if (!sessionId) router.replace('/setup');
  }, [sessionId, router]);

  // Ganti sesi = state lama tidak relevan: reset seluruh store, lalu hidrasi
  // awal dari REST agar peta terisi meski WebSocket belum/gagal tersambung.
  useEffect(() => {
    if (!sessionId) return;
    useSimulationStore.getState().setSessionId(sessionId);
    useTopologyStore.getState().reset();
    useMetricStore.getState().reset();
    useLogStore.getState().reset();
    useFlowStore.getState().reset();
    setPageError(null);

    let cancelled = false;
    (async () => {
      try {
        const [detail, topo, stats] = await Promise.all([
          getSimulation(sessionId),
          getTopology(sessionId),
          getStats(sessionId),
        ]);
        if (cancelled) return;
        useSimulationStore.getState().setSession(detail.session);
        useTopologyStore.getState().applySnapshot(topo);
        useMetricStore.getState().hydrate(stats);
        if (!detail.session.is_active) {
          useSimulationStore.getState().markEnded('sesi ini sudah dihentikan');
        }
      } catch (err) {
        if (cancelled) return;
        setPageError(
          err instanceof ApiError ? err.message : 'tidak dapat memuat sesi dari backend',
        );
      }
      // Skema opsional: sesi tanpa skema mengembalikan 404 — bukan error UI.
      try {
        const schema = await getSchema(sessionId);
        if (!cancelled) {
          useSimulationStore.getState().setSchema(schema.fields, 0);
        }
      } catch {
        /* belum ada skema — abaikan */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  useMonitorSocket(sessionId);

  if (!sessionId) {
    return (
      <main className="grid h-dvh place-items-center text-sm text-slate-400">
        Mengalihkan ke halaman setup…
      </main>
    );
  }

  return (
    <main className="relative h-dvh w-full overflow-hidden">
      <div className="absolute inset-0">
        <MapViewer />
      </div>

      {/* Banner status terminal di atas peta */}
      {(wsStatus === 'ended' || pageError) && (
        <div className="pointer-events-auto absolute left-1/2 top-3 z-[1100] w-[min(92%,560px)] -translate-x-1/2">
          <div className="rounded-lg border border-rose-500/40 bg-rose-950/80 px-4 py-2.5 text-center text-sm text-rose-200 backdrop-blur">
            {pageError ?? `Sesi simulasi berakhir${endedMessage ? ` — ${endedMessage}` : ''}.`}{' '}
            <Link href={`/reports?session=${sessionId}`} className="underline">
              Lihat laporan
            </Link>
            {' · '}
            <Link href="/setup" className="underline">
              Sesi baru
            </Link>
          </div>
        </div>
      )}

      {/* Panel overlay glassmorphism. pointer-events-none pada wadah agar peta
          tetap bisa digeser lewat celah antar kartu.
          Responsif: di ponsel jadi bottom-sheet (dibatasi 62dvh) sehingga bagian
          atas peta tetap terlihat & interaktif; mulai sm jadi kolom kanan penuh
          setinggi layar selebar 400px. */}
      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-[1000] p-3 sm:inset-x-auto sm:inset-y-0 sm:right-0 sm:w-[400px]">
        <div className="thin-scroll flex max-h-[62dvh] flex-col gap-3 overflow-y-auto overscroll-contain pb-1 sm:h-full sm:max-h-none">
          <div className="pointer-events-auto">
            <StatusBar />
          </div>
          <div className="pointer-events-auto">
            <MetricsPanel />
          </div>
          <div className="pointer-events-auto">
            <WeatherControl />
          </div>
          <div className="pointer-events-auto">
            <RadioParamsControl />
          </div>
          <div className="pointer-events-auto">
            <LogStream />
          </div>
          <div className="pointer-events-auto">
            <NodeListPanel />
          </div>
        </div>
      </div>
    </main>
  );
}

// useSearchParams memerlukan boundary Suspense saat prerender App Router.
export default function DashboardPage() {
  return (
    <Suspense
      fallback={
        <main className="grid h-dvh place-items-center text-sm text-slate-400">
          Memuat dasbor…
        </main>
      }
    >
      <DashboardInner />
    </Suspense>
  );
}
