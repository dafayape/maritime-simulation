import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/domain/models/env_params.dart';
import 'package:maritim_node/domain/models/mesh_ack.dart';
import 'package:maritim_node/domain/models/queue_entry.dart';
import 'package:maritim_node/domain/models/routing_info.dart';
import 'package:maritim_node/domain/models/session_summary.dart';
import 'package:maritim_node/domain/models/transmit_frame.dart';

void main() {
  group('Model protokol — bentuk JSON persis events.go backend', () {
    test('EnvParams membaca env:sync_params lengkap', () {
      final env = EnvParams.fromJson({
        'event': 'env:sync_params',
        'sf': 10,
        'max_payload_bytes': 51,
        'ack_timeout_ms': 6200,
        'max_retries': 3,
        'tx_power_dbm': 14,
        'max_range_km': 4.9,
        'weather_severity': 1.5,
      });
      expect(env.spreadingFactor, 10);
      expect(env.maxPayloadBytes, 51);
      expect(env.ackTimeout, const Duration(milliseconds: 6200));
      expect(env.maxRetries, 3);
      expect(env.maxRangeKm, 4.9);
    });

    test('RoutingInfo routed vs isolated', () {
      final routed = RoutingInfo.fromJson({
        'parent_target': 'EDGE-PRATU-01',
        'parent_is_edge': true,
        'distance_to_parent_km': 2.31,
        'hop_level': 1,
        'status': 'routed',
      });
      expect(routed.isRouted, isTrue);
      expect(routed.parentIsEdge, isTrue);

      final isolated = RoutingInfo.fromJson({
        'parent_target': '',
        'parent_is_edge': false,
        'status': 'isolated',
      });
      expect(isolated.isRouted, isFalse);
      expect(const RoutingInfo.isolated().isRouted, isFalse);
    });

    test('TransmitFrame.toTransmitJson memakai nama field kontrak backend',
        () {
      const frame = TransmitFrame(
        packetId: 'pkt-1',
        originNode: 'KPL-001',
        targetParent: 'KPL-002',
        hopCount: 1,
        routingPath: ['KPL-001'],
        binaryPayloadB64: 'AAA=',
      );
      final json = frame.toTransmitJson();
      expect(json['event'], 'node:transmit');
      expect(json['origin_node'], 'KPL-001');
      expect(json['target_parent'], 'KPL-002');
      expect(json['binary_payload_b64'], 'AAA=');
      expect(json['routing_path'], ['KPL-001']);
    });

    test('relayVia mempertahankan origin, menaikkan hop, menambah jalur', () {
      const asli = TransmitFrame(
        packetId: 'pkt-9',
        originNode: 'KPL-004', // pembuat asli (kapal D)
        targetParent: 'KPL-003',
        hopCount: 2,
        routingPath: ['KPL-004', 'KPL-003'],
        binaryPayloadB64: 'AAA=',
      );
      final relay = asli.relayVia(nodeId: 'KPL-002', newParent: 'KPL-001');
      expect(relay.originNode, 'KPL-004',
          reason: 'origin TIDAK BOLEH ditimpa relay');
      expect(relay.hopCount, 3);
      expect(relay.routingPath, ['KPL-004', 'KPL-003', 'KPL-002']);
      expect(relay.targetParent, 'KPL-001');
      expect(relay.packetId, asli.packetId);
      expect(relay.binaryPayloadB64, asli.binaryPayloadB64);
    });

    test('MeshAck membaca duplicate opsional', () {
      final ack = MeshAck.fromJson(
          {'packet_id': 'p', 'receiver_node': 'KPL-2', 'status': 'received'});
      expect(ack.duplicate, isFalse);
      final dup = MeshAck.fromJson({'packet_id': 'p', 'duplicate': true});
      expect(dup.duplicate, isTrue);
    });

    test('SessionSummary membaca baris GET /api/v1/simulations', () {
      final s = SessionSummary.fromJson({
        'id': 'abc-123',
        'session_name': 'uji armada',
        'spreading_factor': 9,
        'tx_power_dbm': 10,
        'weather_severity': 1.0,
        'is_active': true,
        'created_at': '2026-07-03T04:00:00Z',
      });
      expect(s.id, 'abc-123');
      expect(s.isActive, isTrue);
      expect(s.createdAt, isNotNull);
    });

    test('QueueEntry round-trip baris SQLite', () {
      const entry = QueueEntry(
        packetId: 'pkt-x',
        originNodeId: 'KPL-007',
        targetParent: 'KPL-001',
        payloadB64: 'AAA=',
        status: QueueStatus.pending,
        retryCount: 2,
        hopCount: 3,
        routingPath: ['KPL-007', 'KPL-003'],
      );
      final row = entry.toInsertRow('sesi-1');
      expect(row['session_id'], 'sesi-1');
      expect(row['routing_path'], 'KPL-007>KPL-003');

      final back = QueueEntry.fromRow({
        ...row,
        'id': 1,
        'created_at': '2026-07-03T04:00:00Z',
      });
      expect(back.routingPath, ['KPL-007', 'KPL-003']);
      expect(back.status, QueueStatus.pending);
      expect(back.isRelayFor('KPL-001'), isTrue);
      expect(back.isRelayFor('KPL-007'), isFalse);
    });
  });
}
