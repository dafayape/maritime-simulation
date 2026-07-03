/// Arahan routing per-node dari event `mesh:routing_update` (backend
/// `internal/protocol/events.go`, struct `RoutingUpdate`).
///
/// Node hanya mengetahui parent-nya sendiri — meniru memori terbatas ESP32.
/// Rute bersifat *ephemeral*: parent bisa mendadak null (kapal keluar
/// jangkauan ATAU Virtual Edge dihapus dari master data), sehingga status
/// wajib direspons setiap kali event tiba, tidak pernah di-cache permanen
/// (mobile SRS §4B catatan interkoneksi).
class RoutingInfo {
  const RoutingInfo({
    required this.parentTarget,
    required this.parentIsEdge,
    required this.distanceToParentKm,
    required this.hopLevel,
    required this.status,
  });

  factory RoutingInfo.fromJson(Map<String, dynamic> json) => RoutingInfo(
        parentTarget: json['parent_target'] as String? ?? '',
        parentIsEdge: json['parent_is_edge'] as bool? ?? false,
        distanceToParentKm:
            (json['distance_to_parent_km'] as num?)?.toDouble() ?? 0,
        hopLevel: (json['hop_level'] as num?)?.toInt() ?? 0,
        status: json['status'] as String? ?? 'isolated',
      );

  const RoutingInfo.isolated()
      : parentTarget = '',
        parentIsEdge = false,
        distanceToParentKm = 0,
        hopLevel = 0,
        status = 'isolated';

  final String parentTarget;
  final bool parentIsEdge;
  final double distanceToParentKm;
  final int hopLevel;
  final String status;

  bool get isRouted => status == 'routed' && parentTarget.isNotEmpty;

  @override
  String toString() =>
      'RoutingInfo(parent: $parentTarget, edge: $parentIsEdge, status: $status)';
}
