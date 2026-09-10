import { beforeEach, describe, expect, it } from 'vitest';

import { useTopologyStore } from '@/stores/topologyStore';
import type { PacketEvent, RouteEntry } from '@/types/backend';

const route = (partial: Partial<RouteEntry> & { node: string }): RouteEntry => ({
  parent: '',
  parent_is_edge: false,
  distance_km: 0,
  hop_level: 0,
  status: 'isolated',
  ...partial,
});

const packet = (partial: Partial<PacketEvent>): PacketEvent => ({
  event: 'packet:event',
  type: 'transmit',
  packet_id: 'p',
  from_node: 'A',
  to_node: 'B',
  at: new Date().toISOString(),
  ...partial,
});

beforeEach(() => {
  useTopologyStore.getState().reset();
});

describe('TopologyStore', () => {
  it('applyPing membuat node baru (status isolated) lalu menggeser posisinya', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    let node = useTopologyStore.getState().nodes['A-01'];
    expect(node?.status).toBe('isolated');
    expect(node?.lat).toBe(-6.9);

    s.applyPing('A-01', -6.91, 106.52);
    node = useTopologyStore.getState().nodes['A-01'];
    expect(node?.lat).toBe(-6.91);
    expect(node?.lng).toBe(106.52);
  });

  it('applyRoutingTable menetapkan parent/hop dan status routed/isolated', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    s.applyRoutingTable([
      route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed', distance_km: 4.2 }),
      route({ node: 'A-02', status: 'isolated' }),
    ]);
    const nodes = useTopologyStore.getState().nodes;
    expect(nodes['A-01']?.parent).toBe('EDGE-1');
    expect(nodes['A-01']?.parentIsEdge).toBe(true);
    expect(nodes['A-01']?.hopLevel).toBe(1);
    expect(nodes['A-02']?.status).toBe('isolated');
    expect(nodes['A-02']?.parent).toBeNull();
  });

  it('node yang rutenya tidak berubah mempertahankan referensi objek yang sama (anti re-render massal)', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    s.applyPing('A-02', -6.8, 106.4);
    const table = [
      route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed' }),
      route({ node: 'A-02', parent: 'A-01', hop_level: 2, status: 'routed' }),
    ];
    s.applyRoutingTable(table);
    const before = useTopologyStore.getState().nodes;
    s.applyRoutingTable(table); // tabel identik dikirim ulang
    const after = useTopologyStore.getState().nodes;
    expect(after['A-01']).toBe(before['A-01']);
    expect(after['A-02']).toBe(before['A-02']);
  });

  it('drop menandai pengirim retrying; ack/forward memulihkannya', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    s.applyRoutingTable([
      route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed' }),
    ]);

    s.applyPacketEvent(packet({ type: 'drop', from_node: 'A-01', reason: 'air_loss' }));
    expect(useTopologyStore.getState().nodes['A-01']?.linkState).toBe('retrying');

    s.applyPacketEvent(packet({ type: 'ack', from_node: 'A-02', to_node: 'A-01' }));
    expect(useTopologyStore.getState().nodes['A-01']?.linkState).toBe('idle');
  });

  it('routing table baru me-reset linkState retrying (batas staleness)', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    const table = [
      route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed' }),
    ];
    s.applyRoutingTable(table);
    s.applyPacketEvent(packet({ type: 'drop', from_node: 'A-01' }));
    expect(useTopologyStore.getState().nodes['A-01']?.linkState).toBe('retrying');
    s.applyRoutingTable(table);
    expect(useTopologyStore.getState().nodes['A-01']?.linkState).toBe('idle');
  });

  it('setOnline(false) menggugurkan rute jadi isolated; kembali online tak menghapus node', () => {
    const s = useTopologyStore.getState();
    s.applyPing('A-01', -6.9, 106.5);
    s.applyRoutingTable([
      route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed' }),
    ]);
    expect(useTopologyStore.getState().nodes['A-01']?.status).toBe('routed');

    s.setOnline('A-01', false);
    let node = useTopologyStore.getState().nodes['A-01'];
    expect(node?.online).toBe(false);
    expect(node?.status).toBe('isolated'); // tak lagi 'routed' -> RouteLine berhenti menggambar
    expect(node?.parent).toBeNull();
    expect(node?.hopLevel).toBe(0);

    s.setOnline('A-01', true); // rejoin: node tetap ada, tunggu tick routing berikutnya
    node = useTopologyStore.getState().nodes['A-01'];
    expect(node?.online).toBe(true);
    expect(Object.keys(useTopologyStore.getState().nodes)).toContain('A-01');
  });

  it('applySnapshot menggabungkan nodes/routes/edges dan tahan terhadap null (nil slice Go)', () => {
    const s = useTopologyStore.getState();
    s.applySnapshot({ nodes: null, routes: null, edges: null });
    expect(Object.keys(useTopologyStore.getState().nodes)).toHaveLength(0);

    s.applySnapshot({
      nodes: [{ node_id: 'A-01', lat: -6.9, lng: 106.5, online: true }],
      routes: [
        route({ node: 'A-01', parent: 'EDGE-1', parent_is_edge: true, hop_level: 1, status: 'routed' }),
      ],
      edges: [
        {
          id: 1,
          edge_code: 'EDGE-1',
          name: 'Pelabuhan Ratu',
          latitude: -6.9875,
          longitude: 106.5504,
          created_at: '',
          updated_at: '',
        },
      ],
    });
    const state = useTopologyStore.getState();
    expect(state.nodes['A-01']?.status).toBe('routed');
    expect(state.edges['EDGE-1']?.name).toBe('Pelabuhan Ratu');
  });
});
