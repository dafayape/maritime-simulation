/**
 * Cermin (mirror) invarian domain LoRa milik backend Go — murni untuk
 * HINT di UI (validasi form, estimasi byte, label jangkauan radio).
 *
 * Sumber kebenaran tetap backend (`internal/domain/lora.go`, `schema.go`):
 * setiap nilai di sini harus identik dengan konstanta Go-nya, dan angka
 * otoritatif selalu diambil dari respons REST / event `env:sync_params`.
 */

import type { SchemaField } from '@/types/backend';

// --- Batas radio (domain/lora.go) -------------------------------------------

export const MIN_SPREADING_FACTOR = 7;
export const MAX_SPREADING_FACTOR = 12;
export const MIN_TX_POWER_DBM = 2; // batas praktis SX1276
export const MAX_TX_POWER_DBM = 20; // maksimum PA_BOOST SX1276
export const MAX_RETRIES = 3;
export const MIN_WEATHER_SEVERITY = 0.0;
export const MAX_WEATHER_SEVERITY = 2.0;
export const MAX_HOPS = 5;

/** Plafon payload per Spreading Factor: SF7-8 242B, SF9 115B, SF10-12 51B. */
export function maxPayloadBytes(sf: number): number {
  if (sf <= 8) return 242;
  if (sf === 9) return 115;
  return 51;
}

/** Jendela tunggu ACK per SF — kira-kira dua kali lipat tiap kenaikan SF. */
const ACK_TIMEOUTS_MS: Record<number, number> = {
  7: 1_000,
  8: 1_500,
  9: 2_500,
  10: 4_000,
  11: 6_500,
  12: 10_000,
};

export function ackTimeoutMs(sf: number): number {
  const found = ACK_TIMEOUTS_MS[sf];
  if (found !== undefined) return found;
  return sf < MIN_SPREADING_FACTOR
    ? (ACK_TIMEOUTS_MS[MIN_SPREADING_FACTOR] as number)
    : (ACK_TIMEOUTS_MS[MAX_SPREADING_FACTOR] as number);
}

/** Jangkauan radio dari TX power: +6 dB ≈ jarak dua kali (14 dBm ≈ 5 km). */
export function maxRangeKm(txPowerDbm: number): number {
  return 5.0 * 2 ** ((txPowerDbm - 14) / 6);
}

// --- Skema payload dinamis (domain/schema.go) --------------------------------

/** Tipe field fixed-width yang didukung backend + ukuran nominalnya (byte). */
export const FIXED_TYPE_SIZES: Record<string, number> = {
  float32: 4,
  float64: 8,
  int8: 1,
  int16: 2,
  int32: 4,
  int64: 8,
  uint8: 1,
  uint16: 2,
  uint32: 4,
  uint64: 8,
  bool: 1,
};

/** Pilihan tipe untuk dropdown Schema Builder (string_N diinput terpisah). */
export const FIELD_TYPE_OPTIONS = [
  'float32',
  'float64',
  'int8',
  'int16',
  'int32',
  'int64',
  'uint8',
  'uint16',
  'uint32',
  'uint64',
  'bool',
  'string_8',
  'string_16',
  'string_32',
  'string_64',
] as const;

export const MAX_SCHEMA_FIELDS = 32;
const MAX_STRING_FIELD_SIZE = 242;

const FIELD_NAME_RE = /^[a-zA-Z][a-zA-Z0-9_]{0,29}$/;
const STRING_TYPE_RE = /^string_([1-9][0-9]{0,2})$/;

export function isValidFieldName(name: string): boolean {
  return FIELD_NAME_RE.test(name);
}

/** Ukuran data nominal sebuah tipe, atau null jika tipe tidak dikenal. */
export function fieldByteSize(fieldType: string): number | null {
  const fixed = FIXED_TYPE_SIZES[fieldType];
  if (fixed !== undefined) return fixed;
  const m = STRING_TYPE_RE.exec(fieldType);
  if (m) {
    const n = Number(m[1]);
    if (n >= 1 && n <= MAX_STRING_FIELD_SIZE) return n;
  }
  return null;
}

/**
 * Estimasi worst-case ukuran MessagePack sebuah skema — meniru persis
 * domain.EstimatePackedBytes Go: 3 byte header map + per field
 * (1 + len(nama)) byte kunci + (2 + N untuk string_N, 1 + N untuk primitif).
 */
export function estimatePackedBytes(fields: SchemaField[]): number {
  let total = 3;
  for (const f of fields) {
    const size = fieldByteSize(f.type);
    if (size === null) continue;
    total += 1 + f.name.length;
    total += f.type.startsWith('string_') ? 2 + size : 1 + size;
  }
  return total;
}

// --- Nama counter statistik (service/stats.go) --------------------------------

export const STAT_KEYS = {
  transmitTotal: 'transmit_total',
  droppedInvalid: 'dropped_invalid',
  droppedSfLimit: 'dropped_sf_limit',
  droppedMaxHops: 'dropped_max_hops',
  droppedAirLoss: 'dropped_air_loss',
  droppedOutOfRange: 'dropped_out_of_range',
  droppedNoPosition: 'dropped_no_position',
  droppedTargetOffline: 'dropped_target_offline',
  duplicatesFiltered: 'duplicates_filtered',
  forwardedToNode: 'forwarded_to_node',
  deliveredToEdge: 'delivered_to_edge',
  acksRelayed: 'acks_relayed',
  acksDropped: 'acks_dropped',
  acksStale: 'acks_stale',
  pingsAccepted: 'pings_accepted',
  pingsRateLimited: 'pings_rate_limited',
} as const;

/**
 * Pemetaan `reason` pada packet:event type=drop -> nama counter kanonik,
 * mengikuti pemanggilan s.drop(...) di service/simulation_engine.go.
 */
export const DROP_REASON_TO_STAT: Record<string, string> = {
  sf_limit_exceeded: STAT_KEYS.droppedSfLimit,
  max_hops_exceeded: STAT_KEYS.droppedMaxHops,
  transmitter_no_position: STAT_KEYS.droppedNoPosition,
  target_no_position: STAT_KEYS.droppedNoPosition,
  target_offline: STAT_KEYS.droppedTargetOffline,
  air_loss: STAT_KEYS.droppedAirLoss,
  out_of_range: STAT_KEYS.droppedOutOfRange,
};

/** Jumlah seluruh counter drop (prefix `dropped_`). */
export function totalDropped(counters: Record<string, number>): number {
  let sum = 0;
  for (const [key, value] of Object.entries(counters)) {
    if (key.startsWith('dropped_')) sum += value;
  }
  return sum;
}
