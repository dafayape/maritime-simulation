import 'dart:async';
import 'dart:math' as math;

import 'package:geolocator/geolocator.dart';

import '../../domain/models/geo.dart';

/// Sumber koordinat untuk `node:ping` (PRD §4.1): GPS asli perangkat via
/// `geolocator`, atau mode manual dengan random-walk opsional yang meniru
/// pergerakan kapal pada harness `nodesim` backend (drift arah lembut).
class LocationService {
  LocationService({math.Random? random})
      : _random = random ?? math.Random(),
        _heading = (random ?? math.Random()).nextDouble() * 2 * math.pi;

  final math.Random _random;

  GpsMode _mode = GpsMode.manual;
  GeoPoint _current = const GeoPoint(-6.9875, 106.5504);
  bool _drift = false;
  double _speedKmh = 10;
  double _heading;
  StreamSubscription<Position>? _gpsSub;

  GpsMode get mode => _mode;

  GeoPoint get current => _current;

  void setManual({
    required double lat,
    required double lng,
    required bool drift,
    double speedKmh = 10,
  }) {
    _mode = GpsMode.manual;
    _current = GeoPoint(lat, lng);
    _drift = drift;
    _speedKmh = speedKmh;
    _gpsSub?.cancel();
    _gpsSub = null;
  }

  /// Mengaktifkan GPS asli. Mengembalikan pesan kesalahan (Indonesia) bila
  /// layanan mati / izin ditolak — pemanggil menampilkan & jatuh kembali ke
  /// mode manual, aplikasi tidak boleh crash.
  Future<String?> startRealGps() async {
    try {
      if (!await Geolocator.isLocationServiceEnabled()) {
        return 'Layanan lokasi perangkat sedang nonaktif.';
      }
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }
      if (permission == LocationPermission.denied ||
          permission == LocationPermission.deniedForever) {
        return 'Izin lokasi ditolak — gunakan mode koordinat manual.';
      }

      _mode = GpsMode.real;
      await _gpsSub?.cancel();
      _gpsSub = Geolocator.getPositionStream(
        locationSettings:
            const LocationSettings(accuracy: LocationAccuracy.high),
      ).listen(
        (pos) => _current = GeoPoint(pos.latitude, pos.longitude),
        onError: (_) {
          // Sinyal GPS hilang sementara: pertahankan posisi terakhir.
        },
      );

      final last = await Geolocator.getLastKnownPosition();
      if (last != null) {
        _current = GeoPoint(last.latitude, last.longitude);
      }
      return null;
    } on Object catch (e) {
      return 'GPS asli tidak tersedia: $e';
    }
  }

  /// Dipanggil setiap detak ping 3 detik. Pada mode manual dengan drift,
  /// posisi maju satu langkah random-walk (rumus identik `nodesim.move`).
  GeoPoint tick(Duration interval) {
    if (_mode == GpsMode.manual && _drift) {
      final stepKm = _speedKmh * (interval.inMilliseconds / 3600000.0);
      _heading += (_random.nextDouble() - 0.5) * 0.6;
      final dLat = stepKm * math.cos(_heading) / 111.19;
      final dLng = stepKm *
          math.sin(_heading) /
          (111.19 * math.cos(_current.lat * math.pi / 180));
      _current = GeoPoint(_current.lat + dLat, _current.lng + dLng);
    }
    return _current;
  }

  Future<void> dispose() async {
    await _gpsSub?.cancel();
    _gpsSub = null;
  }
}
