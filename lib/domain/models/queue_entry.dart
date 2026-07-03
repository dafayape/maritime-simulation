/// Satu baris antrean Store-and-Forward (tabel `transmit_queue`, SRS §2).
///
/// Kolom `hop_count` dan `routing_path` adalah perluasan dari skema SRS:
/// kontrak transmisi backend mewajibkan kedua field itu ikut terkirim ulang
/// persis saat retry/relay, sehingga wajib dipersistenkan bersama paketnya.
library;

enum QueueStatus {
  pending('PENDING'),
  acked('ACKED'),
  failed('FAILED');

  const QueueStatus(this.dbValue);

  final String dbValue;

  static QueueStatus fromDb(String value) => switch (value) {
        'ACKED' => QueueStatus.acked,
        'FAILED' => QueueStatus.failed,
        _ => QueueStatus.pending,
      };
}

class QueueEntry {
  const QueueEntry({
    required this.packetId,
    required this.originNodeId,
    required this.targetParent,
    required this.payloadB64,
    required this.status,
    required this.retryCount,
    required this.hopCount,
    required this.routingPath,
    this.id,
    this.createdAt,
  });

  factory QueueEntry.fromRow(Map<String, Object?> row) => QueueEntry(
        id: row['id'] as int?,
        packetId: row['packet_id'] as String,
        originNodeId: row['origin_node_id'] as String,
        targetParent: row['target_parent'] as String,
        payloadB64: row['payload_b64'] as String,
        status: QueueStatus.fromDb(row['status'] as String),
        retryCount: (row['retry_count'] as int?) ?? 0,
        hopCount: (row['hop_count'] as int?) ?? 1,
        routingPath: ((row['routing_path'] as String?) ?? '')
            .split('>')
            .where((p) => p.isNotEmpty)
            .toList(),
        createdAt: DateTime.tryParse(row['created_at'] as String? ?? ''),
      );

  final int? id;
  final String packetId;
  final String originNodeId;
  final String targetParent;
  final String payloadB64;
  final QueueStatus status;
  final int retryCount;
  final int hopCount;
  final List<String> routingPath;
  final DateTime? createdAt;

  /// Paket titipan kapal lain (node ini hanya jembatan relay)?
  bool isRelayFor(String myNodeId) => originNodeId != myNodeId;

  Map<String, Object?> toInsertRow(String sessionId) => {
        'session_id': sessionId,
        'packet_id': packetId,
        'origin_node_id': originNodeId,
        'target_parent': targetParent,
        'payload_b64': payloadB64,
        'status': status.dbValue,
        'retry_count': retryCount,
        'hop_count': hopCount,
        'routing_path': routingPath.join('>'),
      };

  QueueEntry copyWith({
    QueueStatus? status,
    int? retryCount,
    String? targetParent,
  }) =>
      QueueEntry(
        id: id,
        packetId: packetId,
        originNodeId: originNodeId,
        targetParent: targetParent ?? this.targetParent,
        payloadB64: payloadB64,
        status: status ?? this.status,
        retryCount: retryCount ?? this.retryCount,
        hopCount: hopCount,
        routingPath: routingPath,
        createdAt: createdAt,
      );
}
