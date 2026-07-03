/// Ringkasan sesi simulasi dari REST `GET /api/v1/simulations` (backend
/// `internal/domain/entities.go`, struct `Session`).
class SessionSummary {
  const SessionSummary({
    required this.id,
    required this.sessionName,
    required this.spreadingFactor,
    required this.txPowerDbm,
    required this.weatherSeverity,
    required this.isActive,
    required this.createdAt,
  });

  factory SessionSummary.fromJson(Map<String, dynamic> json) => SessionSummary(
        id: json['id'] as String? ?? '',
        sessionName: json['session_name'] as String? ?? '',
        spreadingFactor: (json['spreading_factor'] as num?)?.toInt() ?? 7,
        txPowerDbm: (json['tx_power_dbm'] as num?)?.toInt() ?? 10,
        weatherSeverity: (json['weather_severity'] as num?)?.toDouble() ?? 1,
        isActive: json['is_active'] as bool? ?? false,
        createdAt:
            DateTime.tryParse(json['created_at'] as String? ?? '')?.toLocal(),
      );

  final String id;
  final String sessionName;
  final int spreadingFactor;
  final int txPowerDbm;
  final double weatherSeverity;
  final bool isActive;
  final DateTime? createdAt;

  @override
  String toString() => 'SessionSummary($sessionName, aktif: $isActive)';
}
