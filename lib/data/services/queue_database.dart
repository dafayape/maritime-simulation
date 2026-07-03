import 'package:sqflite/sqflite.dart';

import '../../domain/models/queue_entry.dart';

/// Antrean Store-and-Forward persisten (SRS §2, tabel `transmit_queue`).
///
/// Semua mutasi multi-langkah dibungkus `db.transaction()` agar aman dari
/// race condition ketika banyak paket memicu timeout bersamaan (SRS §5.2).
/// Keunikan `packet_id` di tabel sekaligus menjadi mekanisme dedup frame
/// relay: paket yang sama tidak pernah diproses dua kali.
class QueueDatabase {
  QueueDatabase({DatabaseFactory? factory, this._path})
      : _factory = factory ?? databaseFactory;

  final DatabaseFactory _factory;
  final String? _path;
  Database? _db;

  static const _table = 'transmit_queue';

  Future<Database> _open() async {
    final existing = _db;
    if (existing != null && existing.isOpen) return existing;

    final path = _path ??
        '${await _factory.getDatabasesPath()}/maritim_node_queue.db';
    final db = await _factory.openDatabase(
      path,
      options: OpenDatabaseOptions(
        version: 1,
        onCreate: (db, _) async {
          await db.execute('''
            CREATE TABLE $_table (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              session_id TEXT NOT NULL,
              packet_id TEXT UNIQUE NOT NULL,
              origin_node_id TEXT NOT NULL,
              target_parent TEXT NOT NULL,
              payload_b64 TEXT NOT NULL,
              status TEXT NOT NULL,
              retry_count INTEGER DEFAULT 0,
              hop_count INTEGER NOT NULL DEFAULT 1,
              routing_path TEXT NOT NULL DEFAULT '',
              created_at TEXT DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
            )
          ''');
          await db.execute(
            'CREATE INDEX idx_queue_session_status '
            'ON $_table(session_id, status)',
          );
        },
      ),
    );
    _db = db;
    return db;
  }

  /// Menyimpan paket baru berstatus PENDING. Mengembalikan `false` bila
  /// `packet_id` sudah pernah tercatat (duplikat relay — SRS §3C dedup).
  Future<bool> insertPendingIfNew(String sessionId, QueueEntry entry) async {
    final db = await _open();
    return db.transaction((txn) async {
      final dup = await txn.query(
        _table,
        columns: ['id'],
        where: 'packet_id = ?',
        whereArgs: [entry.packetId],
        limit: 1,
      );
      if (dup.isNotEmpty) return false;
      await txn.insert(_table, entry.toInsertRow(sessionId));
      return true;
    });
  }

  Future<void> markAcked(String packetId) async {
    final db = await _open();
    await db.update(
      _table,
      {'status': QueueStatus.acked.dbValue},
      where: 'packet_id = ?',
      whereArgs: [packetId],
    );
  }

  /// Mesin retry (SRS §3B.3) dalam SATU transaksi anti-race:
  /// membaca `retry_count` lalu —
  ///  - bila masih di bawah [maxRetries]: increment dan kembalikan nilai
  ///    barunya (pemanggil menembak ulang payload);
  ///  - bila jatah habis: set status FAILED (Link Lost) dan kembalikan null;
  ///  - bila paket sudah tidak PENDING (misal ACK menang balapan melawan
  ///    timeout): kembalikan null tanpa mengubah apa pun.
  Future<int?> bumpRetryOrFail(String packetId, int maxRetries) async {
    final db = await _open();
    return db.transaction<int?>((txn) async {
      final rows = await txn.query(
        _table,
        columns: ['retry_count', 'status'],
        where: 'packet_id = ?',
        whereArgs: [packetId],
        limit: 1,
      );
      if (rows.isEmpty) return null;
      if (rows.first['status'] != QueueStatus.pending.dbValue) return null;

      final current = (rows.first['retry_count'] as int?) ?? 0;
      if (current >= maxRetries) {
        await txn.update(
          _table,
          {'status': QueueStatus.failed.dbValue},
          where: 'packet_id = ?',
          whereArgs: [packetId],
        );
        return null;
      }
      final next = current + 1;
      await txn.update(
        _table,
        {'retry_count': next},
        where: 'packet_id = ?',
        whereArgs: [packetId],
      );
      return next;
    });
  }

  /// Mengarahkan ulang seluruh paket PENDING ke parent baru — dipanggil saat
  /// `mesh:routing_update` memulihkan rute agar antrean lama ikut pindah
  /// jalur (paket ditahan selama isolated, bukan dibuang).
  Future<void> retargetPending(String sessionId, String newParent) async {
    final db = await _open();
    await db.update(
      _table,
      {'target_parent': newParent},
      where: 'session_id = ? AND status = ?',
      whereArgs: [sessionId, QueueStatus.pending.dbValue],
    );
  }

  Future<List<QueueEntry>> pendingEntries(String sessionId) async {
    final db = await _open();
    final rows = await db.query(
      _table,
      where: 'session_id = ? AND status = ?',
      whereArgs: [sessionId, QueueStatus.pending.dbValue],
      orderBy: 'id ASC',
    );
    return rows.map(QueueEntry.fromRow).toList();
  }

  Future<List<QueueEntry>> recentEntries(String sessionId,
      {int limit = 100}) async {
    final db = await _open();
    final rows = await db.query(
      _table,
      where: 'session_id = ?',
      whereArgs: [sessionId],
      orderBy: 'id DESC',
      limit: limit,
    );
    return rows.map(QueueEntry.fromRow).toList();
  }

  Future<QueueEntry?> byPacketId(String packetId) async {
    final db = await _open();
    final rows = await db.query(
      _table,
      where: 'packet_id = ?',
      whereArgs: [packetId],
      limit: 1,
    );
    return rows.isEmpty ? null : QueueEntry.fromRow(rows.first);
  }

  Future<void> close() async {
    await _db?.close();
    _db = null;
  }
}
