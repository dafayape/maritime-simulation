/// Balasan ACK yang diteruskan server ke pemancar melalui event `mesh:ack`
/// (backend `internal/protocol/events.go`, struct `MeshAck`).
///
/// Catatan kontrak: SRS mobile menamai ACK arah klien→server (`node:ack`)
/// tetapi tidak menamai kaki baliknya; backend mendokumentasikan kaki balik
/// tersebut sebagai `mesh:ack`. `duplicate=true` berarti backend meng-ACK
/// ulang retry yang aslinya sudah sampai — penerima LoRa sungguhan juga
/// membalas duplikat alih-alih diam.
class MeshAck {
  const MeshAck({
    required this.packetId,
    required this.receiverNode,
    required this.status,
    required this.duplicate,
  });

  factory MeshAck.fromJson(Map<String, dynamic> json) => MeshAck(
        packetId: json['packet_id'] as String? ?? '',
        receiverNode: json['receiver_node'] as String? ?? '',
        status: json['status'] as String? ?? 'received',
        duplicate: json['duplicate'] as bool? ?? false,
      );

  final String packetId;
  final String receiverNode;
  final String status;
  final bool duplicate;

  @override
  String toString() =>
      'MeshAck($packetId oleh $receiverNode${duplicate ? ', duplikat' : ''})';
}
