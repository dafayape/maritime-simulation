import { describe, expect, it, vi } from 'vitest';

import { parseMonitorEvent, routeMonitorEvent, type EventSinks } from '@/lib/events';
import type { PacketEvent } from '@/types/backend';

function makeSinks(): EventSinks {
  return {
    topology: {
      applyPing: vi.fn(),
      applyRoutingTable: vi.fn(),
      setOnline: vi.fn(),
      applyPacketEvent: vi.fn(),
    },
    metrics: { applyPacketEvent: vi.fn() },
    logs: { appendPacket: vi.fn(), appendSystem: vi.fn() },
    simulation: { setEnvParams: vi.fn(), setSchema: vi.fn(), markEnded: vi.fn() },
  };
}

describe('parseMonitorEvent', () => {
  it('menerima frame JSON dengan discriminator event yang dikenal', () => {
    const ev = parseMonitorEvent(
      '{"event":"node:ping","node_id":"A-01","session_id":"s","lat":-6.9,"lng":106.5}',
    );
    expect(ev).not.toBeNull();
    expect(ev?.event).toBe('node:ping');
  });

  it('menolak JSON rusak, non-objek, dan event asing', () => {
    expect(parseMonitorEvent('bukan json')).toBeNull();
    expect(parseMonitorEvent('42')).toBeNull();
    expect(parseMonitorEvent('{"event":"alien:event"}')).toBeNull();
    expect(parseMonitorEvent('{"no_event":true}')).toBeNull();
  });
});

describe('routeMonitorEvent', () => {
  it('node:ping menggeser koordinat node di TopologyStore', () => {
    const sinks = makeSinks();
    routeMonitorEvent(
      { event: 'node:ping', node_id: 'A-01', session_id: 's', lat: -6.9, lng: 106.5 },
      sinks,
    );
    expect(sinks.topology.applyPing).toHaveBeenCalledWith('A-01', -6.9, 106.5);
  });

  it('mesh:routing_update meneruskan seluruh tabel rute', () => {
    const sinks = makeSinks();
    const routes = [
      {
        node: 'A-01',
        parent: 'EDGE-PRATU-01',
        parent_is_edge: true,
        distance_km: 3.2,
        hop_level: 1,
        status: 'routed' as const,
      },
    ];
    routeMonitorEvent({ event: 'mesh:routing_update', routes }, sinks);
    expect(sinks.topology.applyRoutingTable).toHaveBeenCalledWith(routes);
  });

  it('packet:event mengalir ke metrik, topologi, dan log sekaligus', () => {
    const sinks = makeSinks();
    const ev: PacketEvent = {
      event: 'packet:event',
      type: 'drop',
      packet_id: 'p-1',
      from_node: 'A-01',
      to_node: 'A-02',
      reason: 'air_loss',
      at: new Date().toISOString(),
    };
    routeMonitorEvent(ev, sinks);
    expect(sinks.metrics.applyPacketEvent).toHaveBeenCalledWith(ev);
    expect(sinks.topology.applyPacketEvent).toHaveBeenCalledWith(ev);
    expect(sinks.logs.appendPacket).toHaveBeenCalledWith(ev);
  });

  it('session:ended menandai sesi berakhir + mencatat log peringatan', () => {
    const sinks = makeSinks();
    routeMonitorEvent(
      { event: 'session:ended', session_id: 's', message: 'dihentikan admin' },
      sinks,
    );
    expect(sinks.simulation.markEnded).toHaveBeenCalledWith('dihentikan admin');
    expect(sinks.logs.appendSystem).toHaveBeenCalled();
  });

  it('node:status online/offline diteruskan ke topologi', () => {
    const sinks = makeSinks();
    routeMonitorEvent({ event: 'node:status', node_id: 'A-03', online: false }, sinks);
    expect(sinks.topology.setOnline).toHaveBeenCalledWith('A-03', false);
  });
});
