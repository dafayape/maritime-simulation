/// Koordinat node dan mode sumber posisi (PRD §4.1 — GPS asli vs manual).
library;

class GeoPoint {
  const GeoPoint(this.lat, this.lng);

  final double lat;
  final double lng;

  @override
  String toString() => '${lat.toStringAsFixed(5)}, ${lng.toStringAsFixed(5)}';
}

enum GpsMode { real, manual }
