'use client';

/**
 * useMonitorSocket — siklus hidup koneksi WebSocket dasbor (SRS §5A).
 *
 * Native HTML5 WebSocket (bukan Socket.io) demi kompatibilitas 100% dengan
 * Gorilla WebSocket di backend Go. Kontrak yang dipegang hook ini:
 *
 *  1. Connect ke  ws://<backend>/ws/monitor?session_id=...
 *  2. Auto-reconnect dengan exponential backoff + jitter saat backend mati,
 *     TAPI berhenti permanen bila sesi memang sudah tidak aktif (dicek via
 *     REST sebelum tiap percobaan, karena browser tidak bisa membaca kode
 *     HTTP 404 dari upgrade WS yang gagal).
 *  3. Saat (re)connect sukses: re-hidrasi state dari REST (sesi + statistik +
 *     snapshot topologi) supaya data yang terlewat selama putus tidak hilang.
 *  4. Polling ringan /stats tiap STATS_POLL_MS: sebagian counter (mis.
 *     `transmit_total` → "Frame Kirim") TIDAK punya packet:event live-nya —
 *     backend hanya menaikkannya di Redis, tak menyiarkannya lewat WS. Tanpa
 *     polling, angka itu beku di nilai saat konek dan "Loss Rate" ikut salah
 *     (0%). Poll menyegarkannya dari sumber kebenaran server secara periodik.
 *  5. Cleanup penuh saat unmount: timer dibatalkan, ws.close(), dan semua
 *     callback dimatikan lewat flag `disposed` — mencegah memory leak dan
 *     setState-terhadap-store untuk sesi yang sudah ditinggalkan.
 */

/** Kadens refresh counter absolut dari REST /stats (ms). */
const STATS_POLL_MS = 4000;

import { useEffect } from 'react';

import { getSimulation, getStats, getTopology } from '@/lib/api';
import { reconnectDelayMs } from '@/lib/backoff';
import { monitorSocketUrl } from '@/lib/config';
import { parseMonitorEvent, routeMonitorEvent, type EventSinks } from '@/lib/events';
import { useFlowStore } from '@/stores/flowStore';
import { useLogStore } from '@/stores/logStore';
import { useMetricStore } from '@/stores/metricStore';
import { useSimulationStore } from '@/stores/simulationStore';
import { useTopologyStore } from '@/stores/topologyStore';

/** Sink produksi: delegasi langsung ke store di luar siklus render React. */
function storeSinks(): EventSinks {
  return {
    topology: {
      applyPing: (id, lat, lng) => useTopologyStore.getState().applyPing(id, lat, lng),
      applyRoutingTable: (routes) => useTopologyStore.getState().applyRoutingTable(routes),
      setOnline: (id, online) => useTopologyStore.getState().setOnline(id, online),
      applyPacketEvent: (ev) => useTopologyStore.getState().applyPacketEvent(ev),
    },
    metrics: {
      applyPacketEvent: (ev) => useMetricStore.getState().applyPacketEvent(ev),
    },
    logs: {
      appendPacket: (ev) => useLogStore.getState().appendPacket(ev),
      appendSystem: (text, level) => useLogStore.getState().appendSystem(text, level),
    },
    simulation: {
      setEnvParams: (params) => useSimulationStore.getState().setEnvParams(params),
      setSchema: (fields, bytes) => useSimulationStore.getState().setSchema(fields, bytes),
      markEnded: (message) => useSimulationStore.getState().markEnded(message),
    },
    // Partikel aliran hanya untuk hop yang sukses (forward/deliver): resolusi
    // posisi asal→tujuan dilakukan di sini (bukan di events.ts yang murni),
    // dengan tujuan bisa berupa node parent maupun Edge (Syahbandar).
    flow: {
      spawnFromPacket: (ev) => {
        if (ev.type !== 'forward' && ev.type !== 'deliver') return;
        const { nodes, edges } = useTopologyStore.getState();
        const from = nodes[ev.from_node];
        if (!from || !Number.isFinite(from.lat) || !Number.isFinite(from.lng)) return;

        const toNode = nodes[ev.to_node];
        let toLat: number;
        let toLng: number;
        if (toNode && Number.isFinite(toNode.lat) && Number.isFinite(toNode.lng)) {
          toLat = toNode.lat;
          toLng = toNode.lng;
        } else {
          const edge = edges[ev.to_node];
          if (!edge) return;
          toLat = edge.latitude;
          toLng = edge.longitude;
        }
        useFlowStore.getState().spawn({
          fromLat: from.lat,
          fromLng: from.lng,
          toLat,
          toLng,
          kind: ev.type,
        });
      },
    },
  };
}

export function useMonitorSocket(sessionId: string | null): void {
  useEffect(() => {
    if (!sessionId) return undefined;

    const sinks = storeSinks();
    const sim = useSimulationStore.getState();
    const logs = useLogStore.getState();

    let disposed = false;
    let ws: WebSocket | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let statsTimer: ReturnType<typeof setInterval> | null = null;
    let attempt = 0;

    // Re-hidrasi via REST setiap kali koneksi (kembali) terbentuk: event yang
    // terlewat selama putus tidak bisa diputar ulang, jadi state absolut
    // (posisi, rute, counter) diambil ulang dari sumber kebenaran server.
    const hydrate = async () => {
      try {
        const [detail, stats, topo] = await Promise.all([
          getSimulation(sessionId),
          getStats(sessionId),
          getTopology(sessionId),
        ]);
        if (disposed) return;
        useSimulationStore.getState().setSession(detail.session);
        useMetricStore.getState().hydrate(stats);
        useTopologyStore.getState().applySnapshot(topo);
      } catch {
        // Backend baru saja menyambung ulang; biarkan stream WS mengisi state.
      }
    };

    const scheduleReconnect = () => {
      if (disposed) return;
      sim.setWsStatus('reconnecting');
      const delay = reconnectDelayMs(attempt++);
      timer = setTimeout(async () => {
        if (disposed) return;
        // Sesi non-aktif membuat endpoint monitor menolak upgrade (404) —
        // hentikan loop reconnect secara permanen alih-alih menyerbu server.
        try {
          const detail = await getSimulation(sessionId);
          if (disposed) return;
          if (!detail.session.is_active) {
            useSimulationStore.getState().markEnded('sesi sudah dihentikan');
            return;
          }
        } catch {
          // REST ikut gagal berarti backend masih down — tetap coba lagi.
        }
        connect();
      }, delay);
    };

    const connect = () => {
      if (disposed) return;
      sim.setWsStatus(attempt === 0 ? 'connecting' : 'reconnecting');

      let socket: WebSocket;
      try {
        socket = new WebSocket(monitorSocketUrl(sessionId));
      } catch {
        scheduleReconnect();
        return;
      }
      ws = socket;

      socket.onopen = () => {
        if (disposed) return;
        const reconnected = attempt > 0;
        attempt = 0;
        sim.setWsStatus('open');
        logs.appendSystem(
          reconnected ? 'WebSocket tersambung kembali' : 'WebSocket monitor tersambung',
        );
        void hydrate();
      };

      socket.onmessage = (msg: MessageEvent) => {
        if (disposed || typeof msg.data !== 'string') return;
        const ev = parseMonitorEvent(msg.data);
        if (ev) routeMonitorEvent(ev, sinks);
      };

      // onerror sengaja tidak menjadwalkan apa pun: spesifikasi WebSocket
      // menjamin onclose selalu menyusul setelah error.
      socket.onerror = () => undefined;

      socket.onclose = () => {
        if (disposed) return;
        if (useSimulationStore.getState().wsStatus === 'ended') return;
        logs.appendSystem('Koneksi WebSocket terputus — mencoba menghubungkan ulang…', 'warn');
        scheduleReconnect();
      };
    };

    connect();

    // Poll counter absolut secara periodik (independen dari WS): counter tanpa
    // packet:event live — terutama transmit_total — hanya bisa mengikuti server
    // lewat REST. Diam saat sesi sudah berakhir agar tidak menyerbu backend.
    statsTimer = setInterval(async () => {
      if (disposed || useSimulationStore.getState().wsStatus === 'ended') return;
      try {
        const stats = await getStats(sessionId);
        if (!disposed) useMetricStore.getState().hydrate(stats);
      } catch {
        // Kegagalan transien diabaikan; poll berikutnya mencoba lagi.
      }
    }, STATS_POLL_MS);

    return () => {
      disposed = true;
      if (timer) clearTimeout(timer);
      if (statsTimer) clearInterval(statsTimer);
      useFlowStore.getState().reset();
      if (ws) {
        // Lepas handler sebelum close agar onclose tidak menjadwalkan reconnect.
        ws.onopen = null;
        ws.onmessage = null;
        ws.onerror = null;
        ws.onclose = null;
        ws.close();
      }
      useSimulationStore.getState().setWsStatus('idle');
    };
  }, [sessionId]);
}
