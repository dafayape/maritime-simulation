import 'dart:math';

/// Preferensi pengguna yang dipersistenkan antar-sesi aplikasi
/// (URL server, identitas node, mode GPS). Nilai default posisi manual
/// berada di sekitar Pelabuhan Ratu — pusat armada seed data backend —
/// supaya node baru langsung berada dalam jangkauan Virtual Edge.
class AppConfig {
  const AppConfig({
    required this.serverUrl,
    required this.nodeId,
    required this.useRealGps,
    required this.manualLat,
    required this.manualLng,
    required this.driftEnabled,
    required this.lastSessionId,
  });

  factory AppConfig.initial() => AppConfig(
        serverUrl: '',
        nodeId: 'KPL-${100 + Random().nextInt(900)}',
        useRealGps: false,
        manualLat: -6.9875,
        manualLng: 106.5504,
        driftEnabled: true,
        lastSessionId: '',
      );

  final String serverUrl;
  final String nodeId;
  final bool useRealGps;
  final double manualLat;
  final double manualLng;
  final bool driftEnabled;
  final String lastSessionId;

  AppConfig copyWith({
    String? serverUrl,
    String? nodeId,
    bool? useRealGps,
    double? manualLat,
    double? manualLng,
    bool? driftEnabled,
    String? lastSessionId,
  }) =>
      AppConfig(
        serverUrl: serverUrl ?? this.serverUrl,
        nodeId: nodeId ?? this.nodeId,
        useRealGps: useRealGps ?? this.useRealGps,
        manualLat: manualLat ?? this.manualLat,
        manualLng: manualLng ?? this.manualLng,
        driftEnabled: driftEnabled ?? this.driftEnabled,
        lastSessionId: lastSessionId ?? this.lastSessionId,
      );
}

/// Pola identitas node yang diterima gateway backend
/// (`internal/ws/gateway.go`: `^[A-Za-z0-9._-]{1,100}$`).
final RegExp nodeIdPattern = RegExp(r'^[A-Za-z0-9._-]{1,100}$');
