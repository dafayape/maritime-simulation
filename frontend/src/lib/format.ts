/** Utilitas format tampilan — murni, tanpa dependensi React. */

import type { PacketEvent } from '@/types/backend';

/** "14:03:21" dari string RFC3339 backend; string kosong bila tak valid. */
export function timeOnly(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleTimeString('id-ID', { hour12: false });
}

export function formatPercent(ratio: number): string {
  if (!Number.isFinite(ratio)) return '0%';
  return `${(ratio * 100).toFixed(1)}%`;
}

export function formatKm(km: number): string {
  return `${km.toFixed(2)} km`;
}

/** UUID panjang -> potongan pendek untuk label ("a1b2c3d4"). */
export function shortId(id: string): string {
  return id.slice(0, 8);
}

/** Label kondisi laut untuk nilai weather severity 0.0–2.0. */
export function weatherLabel(severity: number): string {
  if (severity <= 0.25) return 'Nyaris tanpa gangguan';
  if (severity <= 0.75) return 'Laut tenang';
  if (severity <= 1.25) return 'Normal';
  if (severity <= 1.6) return 'Hujan lebat';
  return 'Badai penuh';
}

/** Terjemahan singkat reason drop backend untuk baris log. */
const DROP_REASON_LABEL: Record<string, string> = {
  air_loss: 'hilang di udara',
  out_of_range: 'di luar jangkauan',
  sf_limit_exceeded: 'payload > batas SF',
  max_hops_exceeded: 'TTL 5 hop habis',
  transmitter_no_position: 'pengirim tanpa posisi',
  target_no_position: 'target tanpa posisi',
  target_offline: 'target offline',
};

export function dropReasonLabel(reason: string | undefined): string {
  if (!reason) return 'drop';
  return DROP_REASON_LABEL[reason] ?? reason;
}

/** Satu baris teks log untuk sebuah packet:event (dipakai LogStream). */
export function describePacketEvent(ev: PacketEvent): string {
  const route = `${ev.from_node} → ${ev.to_node}`;
  switch (ev.type) {
    case 'transmit':
      return `${route} · kirim`;
    case 'forward':
      return `${route} · relay (hop ${ev.hop_count ?? '?'})`;
    case 'deliver':
      return `${route} · SAMPAI di Syahbandar (hop ${ev.hop_count ?? '?'})`;
    case 'drop': {
      const loss =
        ev.loss_probability !== undefined ? ` · P=${ev.loss_probability.toFixed(0)}%` : '';
      return `${route} · DROP: ${dropReasonLabel(ev.reason)}${loss}`;
    }
    case 'duplicate':
      return `${route} · duplikat retry tersaring (dedup)`;
    case 'ack':
      return `${route} · ACK diterima`;
    case 'ack_drop':
      return `${route} · ACK hilang di udara`;
    default:
      return `${route} · ${ev.type}`;
  }
}
