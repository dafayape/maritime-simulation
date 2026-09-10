'use client';

import { useShallow } from 'zustand/react/shallow';

import { NODE_COLOR } from '@/components/map/mapConstants';
import { GlassCard } from '@/components/ui/GlassCard';
import { useTopologyStore, type ShipNode } from '@/stores/topologyStore';
import { useUiStore } from '@/stores/uiStore';

function dotColor(node: ShipNode): string {
  if (!node.online) return NODE_COLOR.offline;
  if (node.status === 'isolated') return NODE_COLOR.isolated;
  if (node.linkState === 'retrying') return NODE_COLOR.retrying;
  return NODE_COLOR.connected;
}

/** Baris kapal — subscribe node-nya sendiri agar daftar tidak render massal. */
function NodeRow({ nodeId }: { nodeId: string }) {
  const node = useTopologyStore((s) => s.nodes[nodeId]);
  const requestFocus = useUiStore((s) => s.requestFocus);
  if (!node) return null;

  return (
    <button
      type="button"
      onClick={() => requestFocus(node.id)}
      className="flex w-full items-center gap-2 rounded-md px-2 py-1 text-left transition hover:bg-slate-800/70"
      title="Terbangkan kamera peta ke kapal ini"
    >
      <span
        className="h-2 w-2 shrink-0 rounded-full"
        style={{ background: dotColor(node), opacity: node.online ? 1 : 0.4 }}
      />
      <span
        className="min-w-0 flex-1 truncate font-mono text-xs text-slate-200"
        title={node.id}
      >
        {node.id}
      </span>
      {!node.online ? (
        <span className="shrink-0 text-[10px] text-slate-400">offline</span>
      ) : node.status === 'routed' ? (
        <span className="shrink-0 font-mono text-[10px] text-slate-400" title={`hop ${node.hopLevel} → ${node.parent}`}>
          hop {node.hopLevel} → {node.parent}
        </span>
      ) : (
        <span className="shrink-0 text-[10px] font-medium text-rose-300">terisolasi</span>
      )}
    </button>
  );
}

export function NodeListPanel() {
  const ids = useTopologyStore(useShallow((s) => Object.keys(s.nodes).sort()));

  return (
    <GlassCard
      title="Armada"
      action={<span className="text-[10px] text-slate-400">{ids.length} kapal</span>}
    >
      <div className="thin-scroll max-h-44 space-y-0.5 overflow-y-auto">
        {ids.length === 0 && (
          <p className="text-xs text-slate-400">
            Belum ada kapal. Jalankan simulator armada (nodesim) atau aplikasi mobile untuk
            bergabung ke sesi ini.
          </p>
        )}
        {ids.map((id) => (
          <NodeRow key={id} nodeId={id} />
        ))}
      </div>
    </GlassCard>
  );
}
