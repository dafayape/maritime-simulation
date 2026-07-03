import 'package:shared_preferences/shared_preferences.dart';

import '../../core/config/app_config.dart';

/// Persistensi preferensi Setup (bukan bagian antrean mesh — data antrean
/// hidup di SQLite, preferensi ringan cukup di SharedPreferences).
class ConfigRepository {
  static const _kServerUrl = 'server_url';
  static const _kNodeId = 'node_id';
  static const _kUseRealGps = 'use_real_gps';
  static const _kManualLat = 'manual_lat';
  static const _kManualLng = 'manual_lng';
  static const _kDrift = 'drift_enabled';
  static const _kLastSession = 'last_session_id';

  Future<AppConfig> load() async {
    final prefs = await SharedPreferences.getInstance();
    final initial = AppConfig.initial();
    return AppConfig(
      serverUrl: prefs.getString(_kServerUrl) ?? initial.serverUrl,
      nodeId: prefs.getString(_kNodeId) ?? initial.nodeId,
      useRealGps: prefs.getBool(_kUseRealGps) ?? initial.useRealGps,
      manualLat: prefs.getDouble(_kManualLat) ?? initial.manualLat,
      manualLng: prefs.getDouble(_kManualLng) ?? initial.manualLng,
      driftEnabled: prefs.getBool(_kDrift) ?? initial.driftEnabled,
      lastSessionId: prefs.getString(_kLastSession) ?? initial.lastSessionId,
    );
  }

  Future<void> save(AppConfig config) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_kServerUrl, config.serverUrl);
    await prefs.setString(_kNodeId, config.nodeId);
    await prefs.setBool(_kUseRealGps, config.useRealGps);
    await prefs.setDouble(_kManualLat, config.manualLat);
    await prefs.setDouble(_kManualLng, config.manualLng);
    await prefs.setBool(_kDrift, config.driftEnabled);
    await prefs.setString(_kLastSession, config.lastSessionId);
  }
}
