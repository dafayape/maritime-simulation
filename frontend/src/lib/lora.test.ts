import { describe, expect, it } from 'vitest';

import {
  ackTimeoutMs,
  estimatePackedBytes,
  fieldByteSize,
  isValidFieldName,
  maxPayloadBytes,
  maxRangeKm,
  totalDropped,
} from '@/lib/lora';

describe('mirror invarian LoRa backend (domain/lora.go)', () => {
  it('plafon payload per SF: 7-8=242B, 9=115B, 10-12=51B', () => {
    expect(maxPayloadBytes(7)).toBe(242);
    expect(maxPayloadBytes(8)).toBe(242);
    expect(maxPayloadBytes(9)).toBe(115);
    expect(maxPayloadBytes(10)).toBe(51);
    expect(maxPayloadBytes(12)).toBe(51);
  });

  it('jendela ACK membesar mengikuti SF (1000ms -> 10000ms)', () => {
    expect(ackTimeoutMs(7)).toBe(1_000);
    expect(ackTimeoutMs(9)).toBe(2_500);
    expect(ackTimeoutMs(12)).toBe(10_000);
    // di luar rentang: clamp ke tepi seperti implementasi Go
    expect(ackTimeoutMs(5)).toBe(1_000);
    expect(ackTimeoutMs(15)).toBe(10_000);
  });

  it('jangkauan: 14 dBm = 5 km, +6 dB melipatgandakan jarak', () => {
    expect(maxRangeKm(14)).toBeCloseTo(5.0, 5);
    expect(maxRangeKm(20)).toBeCloseTo(10.0, 5);
    expect(maxRangeKm(8)).toBeCloseTo(2.5, 5);
  });
});

describe('mirror skema dinamis (domain/schema.go)', () => {
  it('ukuran tipe fixed-width dan string_N', () => {
    expect(fieldByteSize('float32')).toBe(4);
    expect(fieldByteSize('int64')).toBe(8);
    expect(fieldByteSize('bool')).toBe(1);
    expect(fieldByteSize('string_10')).toBe(10);
    expect(fieldByteSize('string_242')).toBe(242);
  });

  it('tipe tidak dikenal atau string di luar batas -> null', () => {
    expect(fieldByteSize('varchar')).toBeNull();
    expect(fieldByteSize('string_0')).toBeNull();
    expect(fieldByteSize('string_243')).toBeNull();
    expect(fieldByteSize('string_')).toBeNull();
  });

  it('validasi nama field mengikuti regex backend', () => {
    expect(isValidFieldName('jenis_ikan')).toBe(true);
    expect(isValidFieldName('Suhu2')).toBe(true);
    expect(isValidFieldName('1abc')).toBe(false);
    expect(isValidFieldName('')).toBe(false);
    expect(isValidFieldName('a'.repeat(31))).toBe(false);
  });

  it('estimasi packed bytes identik dengan EstimatePackedBytes Go', () => {
    // Go: total=3; per field 1+len(nama); string: 2+size; primitif: 1+size.
    const fields = [
      { name: 'jenis_ikan', type: 'string_16' }, // 1+10 + 2+16 = 29
      { name: 'berat_kg', type: 'float32' }, // 1+8 + 1+4 = 14
    ];
    expect(estimatePackedBytes(fields)).toBe(3 + 29 + 14);
  });

  it('field bertipe rusak dilewati tanpa meledakkan estimasi', () => {
    expect(estimatePackedBytes([{ name: 'x', type: 'mystery' }])).toBe(3);
  });
});

describe('agregasi counter drop', () => {
  it('menjumlahkan semua kunci berprefix dropped_', () => {
    expect(
      totalDropped({
        transmit_total: 100,
        dropped_air_loss: 7,
        dropped_sf_limit: 3,
        delivered_to_edge: 90,
      }),
    ).toBe(10);
  });
});
