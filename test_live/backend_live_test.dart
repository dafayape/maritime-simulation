/// Uji integrasi LIVE mobile ↔ backend Go (bukan mock).
///
/// Prasyarat: stack backend berjalan di http://localhost:8080
///   cd ../backend && docker compose up -d
/// Jalankan eksplisit (sengaja di luar folder test/ agar CI unit tidak
/// membutuhkan Docker):
///   flutter test test_live
///
/// Tiga skenario membuktikan seluruh kriteria penerimaan PRD §6 terhadap
/// server sungguhan: (1) originasi → ACK → tercatat di telemetri Syahbandar,
/// (2) relay multi-hop dengan origin_node terjaga, (3) timeout → auto-retry
/// 3x → FAILED saat target mati.
library;

import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:maritim_node/core/config/node_session_config.dart';
import 'package:maritim_node/data/repositories/node_link_repository.dart';
import 'package:maritim_node/data/services/location_service.dart';
import 'package:maritim_node/data/services/node_socket.dart';
import 'package:maritim_node/data/services/queue_database.dart';
import 'package:maritim_node/data/services/rest_client.dart';
import 'package:maritim_node/domain/models/queue_entry.dart';
import 'package:maritim_node/domain/node_link_state.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

const apiBase = 'http://localhost:8080';

/// Uji ini SWASEMBADA terhadap master data: ia membuat Virtual Edge-nya
/// sendiri di perairan Selat Sunda — ratusan km dari edge milik siapa pun —
/// lalu menghapusnya kembali. Dengan begitu hasil uji deterministik meski
/// operator menambah/menghapus edge dari dashboard (edge master data bersifat
/// global dan bisa berubah kapan saja; lihat catatan interkoneksi SRS mobile).
const testEdgeCode = 'EDGE-UJI-MOB';
const testEdgeName = 'Edge Uji Integrasi Mobile';
const edgeLat = -6.30;
const edgeLng = 105.80;
const degPerKmLat = 1 / 111.19;

final List<String> _createdSessions = [];

Future<Map<String, dynamic>> _postJson(
    String path, Map<String, dynamic> body) async {
  final resp = await http.post(
    Uri.parse('$apiBase$path'),
    headers: {'Content-Type': 'application/json'},
    body: jsonEncode(body),
  );
  final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
  if (resp.statusCode >= 300) {
    throw StateError('$path -> ${resp.statusCode}: ${parsed['message']}');
  }
  return parsed;
}

Future<String> _createSession({required int txPower, int sf = 7}) async {
  final resp = await _postJson('/api/v1/simulations', {
    'session_name': 'uji-mobile-live-${DateTime.now().millisecondsSinceEpoch}',
    'spreading_factor': sf,
    'tx_power_dbm': txPower,
    'weather_severity': 1.0,
  });
  final sessionId =
      (resp['data'] as Map<String, dynamic>)['session_id'] as String;
  _createdSessions.add(sessionId);
  await _postJson('/api/v1/simulations/$sessionId/schema', {
    'fields': [
      {'name': 'lat', 'type': 'float32'},
      {'name': 'lng', 'type': 'float32'},
      {'name': 'berat_kg', 'type': 'uint16'},
      {'name': 'jenis_ikan', 'type': 'string_10'},
    ],
  });
  return sessionId;
}

Future<List<Map<String, dynamic>>> _telemetry(String sessionId) async {
  final resp = await http
      .get(Uri.parse('$apiBase/api/v1/simulations/$sessionId/telemetry'));
  final parsed = jsonDecode(resp.body) as Map<String, dynamic>;
  // Amplop: data = { items: [...], total: n } (handleListTelemetry).
  final data = parsed['data'] as Map<String, dynamic>? ?? {};
  return [
    for (final row in (data['items'] as List<dynamic>? ?? []))
      row as Map<String, dynamic>,
  ];
}

/// Node uji lengkap: repository nyata + socket WebSocket nyata + SQLite ffi.
///
/// PENTING: setiap node WAJIB memakai path database unik — factory sqflite
/// meng-cache koneksi per path, sehingga dua node dengan ':memory:' yang
/// sama akan berbagi satu antrean dan saling "melihat" packet_id (dedup
/// relay jadi salah kaprah).
class LiveNode {
  LiveNode(this.nodeId, {required this._lat, required this._lng}) {
    final dir = Directory.systemTemp.createTempSync('maritim-node-$nodeId-');
    _dbDir = dir;
    db = QueueDatabase(factory: databaseFactoryFfi, path: '${dir.path}/queue.db');
    repo = NodeLinkRepository(
      socketFactory: WebSocketNodeSocket.new,
      db: db,
      location: LocationService(),
      restFactory: RestClient.new,
    );
  }

  final String nodeId;
  final double _lat;
  final double _lng;
  late final Directory _dbDir;
  late final QueueDatabase db;
  late final NodeLinkRepository repo;

  Future<void> join(String sessionId) => repo.start(NodeSessionConfig(
        baseUrl: apiBase,
        sessionId: sessionId,
        sessionName: 'live',
        nodeId: nodeId,
        useRealGps: false,
        manualLat: _lat,
        manualLng: _lng,
        driftEnabled: false, // posisi deterministik untuk topologi terprediksi
      ));

  Future<void> shutdown() async {
    await repo.dispose();
    await db.close();
    try {
      _dbDir.deleteSync(recursive: true);
    } on Object {
      // direktori sementara — aman diabaikan bila sudah hilang
    }
  }

  Future<void> waitFor(bool Function(NodeLinkState) cond, String reason,
      {Duration timeout = const Duration(seconds: 25)}) async {
    final end = DateTime.now().add(timeout);
    while (!cond(repo.state)) {
      if (DateTime.now().isAfter(end)) {
        final c = repo.state.counters;
        final logs = repo.state.logs
            .take(10)
            .map((l) => '  [${l.level.name}] ${l.message}')
            .join('\n');
        fail('timeout menunggu: $reason\n'
            'node=$nodeId state=${repo.state.phase} '
            'route=${repo.state.route.status}→${repo.state.route.parentTarget}\n'
            'counters: sent=${c.sent} acked=${c.acked} retried=${c.retried} '
            'failed=${c.failed} relayed=${c.relayed} rf=${c.rfReceived} '
            'dup=${c.duplicates}\n'
            'jurnal terakhir:\n$logs');
      }
      await Future<void>.delayed(const Duration(milliseconds: 150));
    }
  }
}

void main() {
  setUpAll(() async {
    sqfliteFfiInit();
    // Bersihkan sisa run sebelumnya (bila ada) lalu daftarkan edge uji.
    await http.delete(Uri.parse('$apiBase/api/v1/edges/$testEdgeCode'));
    await _postJson('/api/v1/edges', {
      'edge_code': testEdgeCode,
      'name': testEdgeName,
      'latitude': edgeLat,
      'longitude': edgeLng,
    });
  });

  tearDownAll(() async {
    // Hentikan seluruh sesi buatan uji ini agar tidak menumpuk sebagai
    // sesi aktif di server, lalu cabut edge uji dari master data.
    for (final id in _createdSessions) {
      await http.post(Uri.parse('$apiBase/api/v1/simulations/$id/stop'));
    }
    await http.delete(Uri.parse('$apiBase/api/v1/edges/$testEdgeCode'));
  });

  test(
    'Skenario 1 — originasi: connect → env/schema/rute → transmit → mesh:ack '
    '→ tercatat di telemetri Syahbandar',
    () async {
      final sessionId = await _createSession(txPower: 20);
      final node = LiveNode('KPL-M01', lat: edgeLat + 0.005, lng: edgeLng);
      addTearDown(node.shutdown);

      await node.join(sessionId);
      await node.waitFor((s) => s.isOnline, 'kanal online');
      await node.waitFor((s) => s.env != null, 'env:sync_params tiba');
      await node.waitFor((s) => s.schema.isNotEmpty, 'schema:sync tiba');
      await node.waitFor(
          (s) => s.route.isRouted && s.route.parentIsEdge,
          'rute langsung ke Virtual Edge');

      final outcome = await node.repo.transmitUserPayload({
        'lat': edgeLat + 0.005,
        'lng': edgeLng,
        'berat_kg': 120,
        'jenis_ikan': 'tuna',
      });
      expect(outcome.accepted, isTrue, reason: outcome.message);

      await node.waitFor((s) => s.counters.acked >= 1, 'mesh:ack diterima');
      final rows = await node.db.recentEntries(sessionId);
      expect(rows.first.status, QueueStatus.acked);

      // Kebenaran sisi server: payload sampai dan origin tercatat.
      final telemetry = await _telemetry(sessionId);
      expect(telemetry, isNotEmpty,
          reason: 'telemetri Syahbandar harus berisi frame terkirim');
      expect(telemetry.first['origin_node_id'], 'KPL-M01');
      expect(telemetry.first['routing_path'], contains(testEdgeCode));

      final rest = RestClient(apiBase);
      final stats = await rest.stats(sessionId);
      rest.dispose();
      expect((stats['delivered_to_edge'] as num).toInt(),
          greaterThanOrEqualTo(1));
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );

  test(
    'Skenario 2 — relay multi-hop: B di luar jangkauan Edge menitip lewat A; '
    'origin_node tetap milik B sampai Syahbandar',
    () async {
      // tx 10 dBm ≈ 3.1 km. A 2 km utara Edge (dalam jangkauan);
      // B 4.8 km utara Edge (di luar jangkauan Edge, dalam jangkauan A).
      final sessionId = await _createSession(txPower: 10);
      final nodeA =
          LiveNode('KPL-MA', lat: edgeLat + 2.0 * degPerKmLat, lng: edgeLng);
      final nodeB =
          LiveNode('KPL-MB', lat: edgeLat + 4.8 * degPerKmLat, lng: edgeLng);
      addTearDown(nodeA.shutdown);
      addTearDown(nodeB.shutdown);

      await nodeA.join(sessionId);
      await nodeA.waitFor((s) => s.route.isRouted && s.route.parentIsEdge,
          'A terhubung langsung ke Edge');

      await nodeB.join(sessionId);
      await nodeB.waitFor(
          (s) => s.route.isRouted && s.route.parentTarget == 'KPL-MA',
          'B dirutekan menitip lewat A');

      final outcome = await nodeB.repo.transmitUserPayload({
        'lat': edgeLat + 4.8 * degPerKmLat,
        'lng': edgeLng,
        'berat_kg': 77,
        'jenis_ikan': 'cakalang',
      });
      expect(outcome.accepted, isTrue, reason: outcome.message);

      // B harus di-ACK oleh A; A harus meneruskan dan di-ACK Edge.
      await nodeB.waitFor((s) => s.counters.acked >= 1, 'B menerima ACK dari A');
      await nodeA.waitFor((s) => s.counters.relayed >= 1, 'A me-relay frame B');
      await nodeA.waitFor((s) => s.counters.acked >= 1,
          'frame relay A di-ACK oleh Edge');

      // Baris antrean di A wajib mencatat origin B (SRS §3C.2).
      final rowsA = await nodeA.db.recentEntries(sessionId);
      final relayRow =
          rowsA.where((r) => r.originNodeId == 'KPL-MB').toList();
      expect(relayRow, isNotEmpty,
          reason: 'antrean A menyimpan paket titipan milik B');
      expect(relayRow.first.isRelayFor('KPL-MA'), isTrue);
      expect(relayRow.first.hopCount, 2);

      // Kebenaran server: origin = B, jalur = B->A->EDGE.
      final telemetry = await _telemetry(sessionId);
      final fromB = telemetry
          .where((t) => t['origin_node_id'] == 'KPL-MB')
          .toList();
      expect(fromB, isNotEmpty);
      expect(fromB.first['routing_path'], 'KPL-MB->KPL-MA->$testEdgeCode');
      expect((fromB.first['hop_count'] as num).toInt(), greaterThanOrEqualTo(2));
    },
    timeout: const Timeout(Duration(minutes: 3)),
  );

  test(
    'Skenario 3 — Store/Forward/Retry: target mati → timeout ACK → retry 3x '
    'packet_id sama → FAILED (Link Lost)',
    () async {
      final sessionId = await _createSession(txPower: 10);
      final nodeA =
          LiveNode('KPL-RA', lat: edgeLat + 2.0 * degPerKmLat, lng: edgeLng);
      final nodeB =
          LiveNode('KPL-RB', lat: edgeLat + 4.8 * degPerKmLat, lng: edgeLng);
      addTearDown(nodeA.shutdown);
      addTearDown(nodeB.shutdown);

      await nodeA.join(sessionId);
      await nodeA.waitFor((s) => s.route.isRouted, 'A dirutekan');
      await nodeB.join(sessionId);
      await nodeB.waitFor(
          (s) => s.route.isRouted && s.route.parentTarget == 'KPL-RA',
          'B menitip lewat A');

      // Matikan A: server men-drop frame ke target offline TANPA memberi
      // tahu pengirim — persis radio nyata; B hanya tahu dari timeout ACK.
      await nodeA.repo.stop();

      final outcome = await nodeB.repo.transmitUserPayload({
        'lat': edgeLat + 4.8 * degPerKmLat,
        'lng': edgeLng,
        'berat_kg': 55,
        'jenis_ikan': 'layur',
      });
      expect(outcome.accepted, isTrue, reason: outcome.message);

      final maxRetries = nodeB.repo.state.env!.maxRetries;
      await nodeB.waitFor(
        (s) => s.counters.failed >= 1,
        'paket dinyatakan FAILED setelah retry habis',
        timeout: const Duration(seconds: 60),
      );
      expect(nodeB.repo.state.counters.retried, maxRetries,
          reason: 'retry harus tepat $maxRetries kali');

      final rows = await nodeB.db.recentEntries(sessionId);
      final failed =
          rows.where((r) => r.status == QueueStatus.failed).toList();
      expect(failed, isNotEmpty);
      expect(failed.first.retryCount, maxRetries);
    },
    timeout: const Timeout(Duration(minutes: 3)),
  );
}
