/// Amplop frame mesh (backend `internal/domain/packet.go`, struct
/// `TransmitPacket`). Dipakai dua arah:
///  - keluar sebagai `node:transmit` (originasi maupun relay), dan
///  - masuk sebagai `mesh:receive_rf` saat node ini ditunjuk jadi jembatan.
///
/// Aturan server yang WAJIB dipatuhi klien (lihat `HandleTransmit`):
///  - `routing_path` harus diakhiri node yang memancarkan frame ini
///    (pelanggaran = reject `transmitter_mismatch`);
///  - `hop_count` >= 1 dan bertambah satu setiap relay;
///  - `origin_node` tidak pernah ditimpa oleh relay — identitas pembuat
///    asli paket menempel sampai Syahbandar.
class TransmitFrame {
  const TransmitFrame({
    required this.packetId,
    required this.originNode,
    required this.targetParent,
    required this.hopCount,
    required this.routingPath,
    required this.binaryPayloadB64,
  });

  factory TransmitFrame.fromJson(Map<String, dynamic> json) => TransmitFrame(
        packetId: json['packet_id'] as String? ?? '',
        originNode: json['origin_node'] as String? ?? '',
        targetParent: json['target_parent'] as String? ?? '',
        hopCount: (json['hop_count'] as num?)?.toInt() ?? 1,
        routingPath: [
          for (final hop in (json['routing_path'] as List<dynamic>? ?? []))
            hop as String,
        ],
        binaryPayloadB64: json['binary_payload_b64'] as String? ?? '',
      );

  final String packetId;
  final String originNode;
  final String targetParent;
  final int hopCount;
  final List<String> routingPath;
  final String binaryPayloadB64;

  Map<String, dynamic> toTransmitJson() => {
        'event': 'node:transmit',
        'packet_id': packetId,
        'origin_node': originNode,
        'target_parent': targetParent,
        'hop_count': hopCount,
        'routing_path': routingPath,
        'binary_payload_b64': binaryPayloadB64,
      };

  /// Membentuk frame relay: payload dan `origin_node` asli dipertahankan,
  /// `hop_count` naik satu, jalur ditambah identitas node ini, dan target
  /// diarahkan ke parent node ini (SRS §3C langkah 3).
  TransmitFrame relayVia({required String nodeId, required String newParent}) =>
      TransmitFrame(
        packetId: packetId,
        originNode: originNode,
        targetParent: newParent,
        hopCount: hopCount + 1,
        routingPath: [...routingPath, nodeId],
        binaryPayloadB64: binaryPayloadB64,
      );

  @override
  String toString() =>
      'TransmitFrame($packetId asal $originNode → $targetParent, hop $hopCount)';
}
