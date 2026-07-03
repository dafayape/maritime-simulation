/// Parameter runtime satu sesi koneksi node — dirakit SetupScreen dan
/// dikonsumsi mesin data-link saat `start()`.
class NodeSessionConfig {
  const NodeSessionConfig({
    required this.baseUrl,
    required this.sessionId,
    required this.sessionName,
    required this.nodeId,
    required this.useRealGps,
    required this.manualLat,
    required this.manualLng,
    required this.driftEnabled,
  });

  final String baseUrl;
  final String sessionId;
  final String sessionName;
  final String nodeId;
  final bool useRealGps;
  final double manualLat;
  final double manualLng;
  final bool driftEnabled;
}
