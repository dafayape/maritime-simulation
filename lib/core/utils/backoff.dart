/// Exponential backoff untuk auto-reconnect WebSocket (SRS §5.3):
/// 1s → 2s → 4s → 8s → 16s → 30s (plafon), di-reset saat koneksi berhasil.
class ExponentialBackoff {
  ExponentialBackoff({
    this.initial = const Duration(seconds: 1),
    this.cap = const Duration(seconds: 30),
  });

  final Duration initial;
  final Duration cap;

  int _attempt = 0;

  /// Nomor percobaan berikutnya (1-based setelah [next] pertama dipanggil).
  int get attempt => _attempt;

  Duration next() {
    final factor = 1 << (_attempt > 30 ? 30 : _attempt);
    _attempt++;
    final delay = initial * factor;
    return delay > cap ? cap : delay;
  }

  void reset() => _attempt = 0;
}
