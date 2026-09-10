/**
 * TopologyStore (SRS §2A): posisi absolut kapal + relasi mesh routing.
 *
 * Struktur `Record<string, ShipNode>` (lookup O(1)) dipilih alih-alih Array
 * sesuai SRS, dan setiap aksi HANYA mengganti objek node yang benar-benar
 * berubah — node lain tetap referensi lama sehingga selector per-node di
 * komponen marker tidak memicu re-render massal saat satu kapal bergerak.
 */

import { create } from 'zustand';

import type {
  Edge,
  PacketEvent,
  RouteEntry,
  RouteStatus,
  TopologySnapshot,
} from '@/types/backend';

export type NodeLinkState = 'idle' | 'retrying';

export interface ShipNode {
  id: string;
  /** NaN selama posisi belum diketahui (route tiba sebelum ping pertama). */
  lat: number;
  lng: number;
  online: boolean;
  parent: string | null;
  parentIsEdge: boolean;
  distanceKm: number;
  hopLevel: number;
  status: RouteStatus;
  linkState: NodeLinkState;
  /** Epoch ms sentuhan WS terakhir — ditampilkan di popup marker. */
  lastEventAt: number;
}

export interface TopologyState {
  nodes: Record<string, ShipNode>;
  /** Edge (Syahbandar) di-key dengan edge_code — kunci yang dirujuk parent rute. */
  edges: Record<string, Edge>;
  applySnapshot(snap: TopologySnapshot): void;
  applyRoutingTable(routes: RouteEntry[] | null): void;
  applyPing(nodeId: string, lat: number, lng: number): void;
  setOnline(nodeId: string, online: boolean): void;
  applyPacketEvent(ev: PacketEvent): void;
  setEdges(edges: Edge[] | null): void;
  reset(): void;
}

function blankNode(id: string): ShipNode {
  // Node yang baru dikenal dianggap isolated (belum punya rute) sampai tick
  // topologi backend berikutnya (≤5 s) menetapkan parent-nya.
  return {
    id,
    lat: Number.NaN,
    lng: Number.NaN,
    online: true,
    parent: null,
    parentIsEdge: false,
    distanceKm: 0,
    hopLevel: 0,
    status: 'isolated',
    linkState: 'idle',
    lastEventAt: Date.now(),
  };
}

function routeChanged(node: ShipNode, route: RouteEntry): boolean {
  return (
    node.parent !== (route.parent || null) ||
    node.parentIsEdge !== route.parent_is_edge ||
    node.distanceKm !== route.distance_km ||
    node.hopLevel !== route.hop_level ||
    node.status !== route.status ||
    node.linkState !== 'idle'
  );
}

export const useTopologyStore = create<TopologyState>()((set) => ({
  nodes: {},
  edges: {},

  applySnapshot: (snap) =>
    set((state) => {
      const nodes: Record<string, ShipNode> = { ...state.nodes };
      for (const n of snap.nodes ?? []) {
        const prev = nodes[n.node_id] ?? blankNode(n.node_id);
        nodes[n.node_id] = { ...prev, lat: n.lat, lng: n.lng, online: n.online };
      }
      const edges: Record<string, Edge> = { ...state.edges };
      for (const e of snap.edges ?? []) {
        edges[e.edge_code] = e;
      }
      return { nodes: applyRoutes(nodes, snap.routes), edges };
    }),

  applyRoutingTable: (routes) =>
    set((state) => ({ nodes: applyRoutes({ ...state.nodes }, routes) })),

  applyPing: (nodeId, lat, lng) =>
    set((state) => {
      const prev = state.nodes[nodeId] ?? blankNode(nodeId);
      return {
        nodes: {
          ...state.nodes,
          [nodeId]: { ...prev, lat, lng, online: true, lastEventAt: Date.now() },
        },
      };
    }),

  // node:status. Saat kapal PUTUS, backend menghapus posisinya dari geo-index
  // sehingga rutenya tak lagi valid: kita gugurkan parent/hop dan tandai
  // isolated agar (1) garis rute hantu ke kapal mati langsung hilang
  // (RouteLine hanya menggambar status 'routed') dan (2) data topologi jujur.
  // Saat kapal kembali online, tick routing backend berikutnya (≤5 s) yang
  // menetapkan ulang rutenya — di sini cukup nyalakan flag online.
  setOnline: (nodeId, online) =>
    set((state) => {
      const prev = state.nodes[nodeId] ?? blankNode(nodeId);
      const next: ShipNode = online
        ? { ...prev, online: true, lastEventAt: Date.now() }
        : {
            ...prev,
            online: false,
            status: 'isolated',
            parent: null,
            parentIsEdge: false,
            hopLevel: 0,
            distanceKm: 0,
            linkState: 'idle',
            lastEventAt: Date.now(),
          };
      return { nodes: { ...state.nodes, [nodeId]: next } };
    }),

  // Derivasi status "retrying" (kuning) dari aliran packet:event:
  //  - drop / ack_drop / duplicate  -> pengirim sedang menunggu-ulang ACK,
  //  - forward / deliver / ack      -> hop sukses, kembali idle.
  // Tidak memakai timer buatan (larangan "no faked delays"); staleness
  // dibatasi oleh applyRoutingTable yang me-reset linkState tiap tick rute.
  applyPacketEvent: (ev) =>
    set((state) => {
      let target: string | null = null;
      let linkState: NodeLinkState = 'idle';
      switch (ev.type) {
        case 'drop':
        case 'duplicate':
          target = ev.from_node;
          linkState = 'retrying';
          break;
        case 'ack_drop':
          // ACK gagal kembali: node tujuan ACK (transmitter awal) masih menunggu.
          target = ev.to_node;
          linkState = 'retrying';
          break;
        case 'forward':
        case 'deliver':
          target = ev.from_node;
          linkState = 'idle';
          break;
        case 'ack':
          target = ev.to_node;
          linkState = 'idle';
          break;
        default:
          return state;
      }
      const prev = target ? state.nodes[target] : undefined;
      if (!prev || prev.linkState === linkState) return state;
      return {
        nodes: {
          ...state.nodes,
          [prev.id]: { ...prev, linkState, lastEventAt: Date.now() },
        },
      };
    }),

  setEdges: (edgeList) =>
    set((state) => {
      const edges: Record<string, Edge> = { ...state.edges };
      for (const e of edgeList ?? []) {
        edges[e.edge_code] = e;
      }
      return { edges };
    }),

  reset: () => set({ nodes: {}, edges: {} }),
}));

/**
 * Terapkan tabel routing penuh dari backend ke peta node secara in-place
 * (caller sudah menyalin Record-nya). Objek node hanya diganti bila field
 * rutenya benar-benar berubah — kunci performa render 50+ kapal.
 */
function applyRoutes(
  nodes: Record<string, ShipNode>,
  routes: RouteEntry[] | null,
): Record<string, ShipNode> {
  for (const route of routes ?? []) {
    const prev = nodes[route.node] ?? blankNode(route.node);
    if (nodes[route.node] && !routeChanged(prev, route)) continue;
    nodes[route.node] = {
      ...prev,
      parent: route.parent || null,
      parentIsEdge: route.parent_is_edge,
      distanceKm: route.distance_km,
      hopLevel: route.hop_level,
      status: route.status,
      linkState: 'idle',
    };
  }
  return nodes;
}
