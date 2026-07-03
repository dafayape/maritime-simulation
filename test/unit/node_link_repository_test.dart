import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/core/config/node_session_config.dart';
import 'package:maritim_node/data/repositories/node_link_repository.dart';
import 'package:maritim_node/data/services/location_service.dart';
import 'package:maritim_node/data/services/queue_database.dart';
import 'package:maritim_node/domain/models/env_params.dart';
import 'package:maritim_node/domain/models/mesh_ack.dart';
import 'package:maritim_node/domain/models/queue_entry.dart';
import 'package:maritim_node/domain/models/routing_info.dart';
import 'package:maritim_node/domain/models/schema_field.dart';
import 'package:maritim_node/domain/models/transmit_frame.dart';
import 'package:maritim_node/domain/node_event.dart';
import 'package:maritim_node/domain/node_link_state.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

import '../helpers/fake_node_socket.dart';

const _cfg = NodeSessionConfig(
  baseUrl: 'http://localhost:8080',
  sessionId: 'sesi-uji',
  sessionName: 'Uji Unit',
  nodeId: 'KPL-100',
  useRealGps: false,
  manualLat: -6.9875,
  manualLng: 106.5504,
  driftEnabled: false,
);

/// Timeout ACK super pendek supaya seluruh siklus retry selesai < 1 detik.
const _env = EnvParams(
  spreadingFactor: 7,
  maxPayloadBytes: 242,
  ackTimeoutMs: 40,
  maxRetries: 3,
  txPowerDbm: 10,
  maxRangeKm: 3.1,
  weatherSeverity: 1,
);

const _routed = RoutingInfo(
  parentTarget: 'KPL-001',
  parentIsEdge: false,
  distanceToParentKm: 1.2,
  hopLevel: 2,
  status: 'routed',
);

void main() {
  setUpAll(sqfliteFfiInit);

  late FakeNodeSocket socket;
  late QueueDatabase db;
  late NodeLinkRepository repo;

  setUp(() {
    socket = FakeNodeSocket();
    db = QueueDatabase(factory: databaseFactoryFfi, path: inMemoryDatabasePath);
    repo = NodeLinkRepository(
      socketFactory: () => socket,
      db: db,
      location: LocationService(),
      ackGrace: Duration.zero,
    );
  });

  tearDown(() async {
    await repo.dispose();
    await db.close();
  });

  /// Menyiapkan node online lengkap: env + skema + rute.
  Future<void> bringOnline() async {
    await repo.start(_cfg);
    socket
      ..emit(const EnvUpdated(_env))
      ..emit(const SchemaUpdated(
          [SchemaField(name: 'berat_kg', type: 'uint16')], 12))
      ..emit(const RouteUpdated(_routed));
    await pumpEventQueue();
  }

  group('NodeLinkRepository — mesin Data Link', () {
    test('connect memancarkan node:ping segera (registrasi posisi)',
        () async {
      await bringOnline();
      final pings = socket.framesOf('node:ping');
      expect(pings, isNotEmpty);
      expect(pings.first['node_id'], 'KPL-100');
      expect(pings.first['session_id'], 'sesi-uji');
      expect(pings.first['lat'], closeTo(-6.9875, 1e-6));
      expect(repo.state.phase, LinkPhase.online);
    });

    test('transmit: frame sesuai kontrak + PENDING + ACK → ACKED', () async {
      await bringOnline();
      final outcome = await repo.transmitUserPayload({'berat_kg': 120});
      expect(outcome.accepted, isTrue);
      await pumpEventQueue();

      final tx = socket.framesOf('node:transmit');
      expect(tx, hasLength(1));
      expect(tx.first['origin_node'], 'KPL-100');
      expect(tx.first['target_parent'], 'KPL-001');
      expect(tx.first['hop_count'], 1);
      expect(tx.first['routing_path'], ['KPL-100'],
          reason: 'routing_path harus diakhiri pemancar (aturan server)');

      final packetId = tx.first['packet_id'] as String;
      expect((await db.byPacketId(packetId))!.status, QueueStatus.pending);

      socket.emit(AckReceived(MeshAck(
          packetId: packetId,
          receiverNode: 'KPL-001',
          status: 'received',
          duplicate: false)));
      await pumpEventQueue();

      expect((await db.byPacketId(packetId))!.status, QueueStatus.acked);
      expect(repo.state.counters.acked, 1);
      expect(repo.state.counters.failed, 0);
    });

    test(
        'tanpa ACK: retry 3x dengan packet_id SAMA lalu FAILED (SRS §3B)',
        () async {
      await bringOnline();
      await repo.transmitUserPayload({'berat_kg': 42});
      await pumpEventQueue();

      // 1 kirim awal + 3 retry × (40ms + antre) — beri ruang 800ms.
      await Future<void>.delayed(const Duration(milliseconds: 800));

      final tx = socket.framesOf('node:transmit');
      expect(tx, hasLength(4), reason: '1 percobaan awal + 3 retry');
      final ids = tx.map((f) => f['packet_id']).toSet();
      expect(ids, hasLength(1),
          reason: 'retry wajib memakai packet_id yang sama (dedup server)');

      final row = await db.byPacketId(ids.first as String);
      expect(row!.status, QueueStatus.failed);
      expect(row.retryCount, 3);
      expect(repo.state.counters.retried, 3);
      expect(repo.state.counters.failed, 1);
    });

    test('mesh:receive_rf: ACK seketika, origin dipertahankan, forward hop+1',
        () async {
      await bringOnline();
      const incoming = TransmitFrame(
        packetId: 'pkt-asing-1',
        originNode: 'KPL-999',
        targetParent: 'KPL-100',
        hopCount: 1,
        routingPath: ['KPL-999'],
        binaryPayloadB64: 'kaJrZwE=',
      );
      socket.emit(const RfReceived(incoming));
      await pumpEventQueue();

      final acks = socket.framesOf('node:ack');
      expect(acks, hasLength(1));
      expect(acks.first['packet_id'], 'pkt-asing-1');
      expect(acks.first['receiver_node'], 'KPL-100');

      final tx = socket.framesOf('node:transmit');
      expect(tx, hasLength(1));
      expect(tx.first['origin_node'], 'KPL-999',
          reason: 'origin pembuat asli TIDAK ditimpa identitas relay');
      expect(tx.first['hop_count'], 2);
      expect(tx.first['routing_path'], ['KPL-999', 'KPL-100']);
      expect(tx.first['target_parent'], 'KPL-001');

      final row = await db.byPacketId('pkt-asing-1');
      expect(row!.originNodeId, 'KPL-999');
      expect(repo.state.counters.relayed, 1);

      // Duplikat frame yang sama: tetap di-ACK, tidak diteruskan lagi.
      socket.emit(const RfReceived(incoming));
      await pumpEventQueue();
      expect(socket.framesOf('node:ack'), hasLength(2));
      expect(socket.framesOf('node:transmit'), hasLength(1));
      expect(repo.state.counters.duplicates, 1);
    });

    test(
        'terisolasi: paket ditahan PENDING, lalu dipompa + retarget saat '
        'rute pulih (Store-and-Forward)', () async {
      await repo.start(_cfg);
      socket
        ..emit(const EnvUpdated(_env))
        ..emit(const RouteUpdated(RoutingInfo.isolated()));
      await pumpEventQueue();

      final outcome = await repo.transmitUserPayload({'bebas': 1});
      expect(outcome.accepted, isTrue);
      await pumpEventQueue();
      expect(socket.framesOf('node:transmit'), isEmpty,
          reason: 'tanpa rute tidak boleh ada frame mengudara');

      socket.emit(const RouteUpdated(_routed));
      await pumpEventQueue();
      await Future<void>.delayed(const Duration(milliseconds: 30));

      final tx = socket.framesOf('node:transmit');
      expect(tx, hasLength(1), reason: 'antrean dipompa saat rute pulih');
      expect(tx.first['target_parent'], 'KPL-001',
          reason: 'paket lama di-retarget ke parent baru');
    });

    test('payload melebihi limit SF ditolak sebelum menyentuh antrean',
        () async {
      await bringOnline();
      socket.emit(EnvUpdated(EnvParams.fromJson(const {
        'sf': 12,
        'max_payload_bytes': 4,
        'ack_timeout_ms': 40,
        'max_retries': 3,
      })));
      await pumpEventQueue();

      final outcome =
          await repo.transmitUserPayload({'berat_kg': 500});
      expect(outcome.accepted, isFalse);
      expect(outcome.message, contains('limit'));
      expect(socket.framesOf('node:transmit'), isEmpty);
      expect(await db.pendingEntries('sesi-uji'), isEmpty);
    });

    test('session:ended menghentikan mesin tanpa reconnect', () async {
      await bringOnline();
      socket.emit(const SessionEndedEvent('Simulasi dihentikan.'));
      await pumpEventQueue();
      expect(repo.state.phase, LinkPhase.ended);
      expect(repo.state.endedReason, contains('dihentikan'));

      final outcome = await repo.transmitUserPayload({'berat_kg': 1});
      expect(outcome.accepted, isFalse);
    });
  });
}
