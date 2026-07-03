import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/data/services/queue_database.dart';
import 'package:maritim_node/domain/models/queue_entry.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

QueueEntry _entry(String packetId,
        {String origin = 'KPL-001', String target = 'KPL-002'}) =>
    QueueEntry(
      packetId: packetId,
      originNodeId: origin,
      targetParent: target,
      payloadB64: 'AAA=',
      status: QueueStatus.pending,
      retryCount: 0,
      hopCount: 1,
      routingPath: [origin],
    );

void main() {
  setUpAll(sqfliteFfiInit);

  late QueueDatabase db;

  setUp(() {
    db = QueueDatabase(factory: databaseFactoryFfi, path: inMemoryDatabasePath);
  });

  tearDown(() => db.close());

  group('QueueDatabase — Store-and-Forward (SRS §2)', () {
    test('insertPendingIfNew menolak packet_id ganda (dedup relay)', () async {
      expect(await db.insertPendingIfNew('s1', _entry('pkt-1')), isTrue);
      expect(await db.insertPendingIfNew('s1', _entry('pkt-1')), isFalse);
      expect((await db.pendingEntries('s1')).length, 1);
    });

    test('bumpRetryOrFail: increment 1..max lalu FAILED (Link Lost)',
        () async {
      await db.insertPendingIfNew('s1', _entry('pkt-1'));
      expect(await db.bumpRetryOrFail('pkt-1', 3), 1);
      expect(await db.bumpRetryOrFail('pkt-1', 3), 2);
      expect(await db.bumpRetryOrFail('pkt-1', 3), 3);
      expect(await db.bumpRetryOrFail('pkt-1', 3), isNull);

      final row = await db.byPacketId('pkt-1');
      expect(row!.status, QueueStatus.failed);
      expect(row.retryCount, 3);
    });

    test('bumpRetryOrFail tidak menyentuh paket yang sudah ACKED '
        '(ACK menang balapan melawan timeout)', () async {
      await db.insertPendingIfNew('s1', _entry('pkt-1'));
      await db.markAcked('pkt-1');
      expect(await db.bumpRetryOrFail('pkt-1', 3), isNull);
      expect((await db.byPacketId('pkt-1'))!.status, QueueStatus.acked);
    });

    test('timeout serentak tetap konsisten (transaksi anti-race, SRS §5.2)',
        () async {
      await db.insertPendingIfNew('s1', _entry('pkt-1'));
      final results = await Future.wait(
          [for (var i = 0; i < 6; i++) db.bumpRetryOrFail('pkt-1', 3)]);
      // Tepat 3 increment sukses; sisanya null (FAILED / sudah final).
      expect(results.whereType<int>().toList()..sort(), [1, 2, 3]);
      final row = await db.byPacketId('pkt-1');
      expect(row!.status, QueueStatus.failed);
      expect(row.retryCount, 3);
    });

    test('retargetPending hanya memindahkan paket PENDING sesi tersebut',
        () async {
      await db.insertPendingIfNew('s1', _entry('pkt-1'));
      await db.insertPendingIfNew('s1', _entry('pkt-2'));
      await db.insertPendingIfNew('s2', _entry('pkt-3'));
      await db.markAcked('pkt-2');

      await db.retargetPending('s1', 'KPL-099');

      expect((await db.byPacketId('pkt-1'))!.targetParent, 'KPL-099');
      expect((await db.byPacketId('pkt-2'))!.targetParent, 'KPL-002',
          reason: 'paket ACKED tidak boleh berubah');
      expect((await db.byPacketId('pkt-3'))!.targetParent, 'KPL-002',
          reason: 'sesi lain tidak boleh tersentuh');
    });

    test('recentEntries urut terbaru dahulu dan terbatas', () async {
      for (var i = 1; i <= 5; i++) {
        await db.insertPendingIfNew('s1', _entry('pkt-$i'));
      }
      final recent = await db.recentEntries('s1', limit: 3);
      expect(recent.map((e) => e.packetId), ['pkt-5', 'pkt-4', 'pkt-3']);
    });

    test('origin_node_id relay tersimpan apa adanya (SRS §3C.2)', () async {
      await db.insertPendingIfNew(
          's1', _entry('pkt-r', origin: 'KPL-777', target: 'KPL-001'));
      final row = await db.byPacketId('pkt-r');
      expect(row!.originNodeId, 'KPL-777');
      expect(row.isRelayFor('KPL-001'), isTrue);
    });
  });
}
