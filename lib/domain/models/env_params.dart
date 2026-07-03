/// Parameter lingkungan radio yang dipancarkan backend lewat event
/// `env:sync_params` (kontrak: backend `internal/protocol/events.go`,
/// struct `EnvSyncParams`).
///
/// Klien sengaja dibuat "bodoh": batas byte, timeout ACK, dan jatah retry
/// semuanya didikte peladen — meniru firmware ESP32 yang menerima konfigurasi
/// dari gateway, bukan menghitung sendiri.
class EnvParams {
  const EnvParams({
    required this.spreadingFactor,
    required this.maxPayloadBytes,
    required this.ackTimeoutMs,
    required this.maxRetries,
    required this.txPowerDbm,
    required this.maxRangeKm,
    required this.weatherSeverity,
  });

  factory EnvParams.fromJson(Map<String, dynamic> json) => EnvParams(
        spreadingFactor: (json['sf'] as num?)?.toInt() ?? 7,
        maxPayloadBytes: (json['max_payload_bytes'] as num?)?.toInt() ?? 242,
        ackTimeoutMs: (json['ack_timeout_ms'] as num?)?.toInt() ?? 2000,
        maxRetries: (json['max_retries'] as num?)?.toInt() ?? 3,
        txPowerDbm: (json['tx_power_dbm'] as num?)?.toInt() ?? 10,
        maxRangeKm: (json['max_range_km'] as num?)?.toDouble() ?? 0,
        weatherSeverity: (json['weather_severity'] as num?)?.toDouble() ?? 1,
      );

  final int spreadingFactor;
  final int maxPayloadBytes;
  final int ackTimeoutMs;
  final int maxRetries;
  final int txPowerDbm;
  final double maxRangeKm;
  final double weatherSeverity;

  Duration get ackTimeout => Duration(milliseconds: ackTimeoutMs);

  @override
  String toString() =>
      'EnvParams(sf: $spreadingFactor, limit: ${maxPayloadBytes}B, '
      'ackTimeout: ${ackTimeoutMs}ms, retries: $maxRetries)';
}
