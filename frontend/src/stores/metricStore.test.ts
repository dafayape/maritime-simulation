import { beforeEach, describe, expect, it } from 'vitest';

import { statKeyForPacketEvent, useMetricStore } from '@/stores/metricStore';
import type { PacketEvent } from '@/types/backend';

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
  useMetricStore.getState().reset();
});

describe('statKeyForPacketEvent', () => {
  it('memetakan tipe event ke nama counter kanonik backend', () => {
    expect(statKeyForPacketEvent(packet({ type: 'transmit' }))).toBe('transmit_total');
    expect(statKeyForPacketEvent(packet({ type: 'forward' }))).toBe('forwarded_to_node');
    expect(statKeyForPacketEvent(packet({ type: 'deliver' }))).toBe('delivered_to_edge');
    expect(statKeyForPacketEvent(packet({ type: 'duplicate' }))).toBe('duplicates_filtered');
    expect(statKeyForPacketEvent(packet({ type: 'ack' }))).toBe('acks_relayed');
    expect(statKeyForPacketEvent(packet({ type: 'ack_drop' }))).toBe('acks_dropped');
  });

  it('drop memakai reason untuk memilih counter dropped_* yang tepat', () => {
    expect(statKeyForPacketEvent(packet({ type: 'drop', reason: 'air_loss' }))).toBe(
      'dropped_air_loss',
    );
    expect(statKeyForPacketEvent(packet({ type: 'drop', reason: 'out_of_range' }))).toBe(
      'dropped_out_of_range',
    );
    expect(statKeyForPacketEvent(packet({ type: 'drop', reason: 'sf_limit_exceeded' }))).toBe(
      'dropped_sf_limit',
    );
  });
});

describe('MetricStore', () => {
  it('hydrate menimpa nilai absolut, packet:event menaikkan counter yang sama', () => {
    const s = useMetricStore.getState();
    s.hydrate({ transmit_total: 100, delivered_to_edge: 60 });
    s.applyPacketEvent(packet({ type: 'transmit' }));
    s.applyPacketEvent(packet({ type: 'deliver' }));
    const counters = useMetricStore.getState().counters;
    expect(counters.transmit_total).toBe(101);
    expect(counters.delivered_to_edge).toBe(61);
  });

  it('hydrate(null) tidak menghapus counter yang sudah berjalan', () => {
    const s = useMetricStore.getState();
    s.applyPacketEvent(packet({ type: 'transmit' }));
    s.hydrate(null);
    expect(useMetricStore.getState().counters.transmit_total).toBe(1);
  });
});
