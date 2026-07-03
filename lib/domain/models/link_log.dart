/// Entri jurnal Data Link untuk panel Log di HUD.
library;

enum LogLevel { info, ok, warn, error }

class LinkLog {
  LinkLog(this.level, this.message) : at = DateTime.now();

  final DateTime at;
  final LogLevel level;
  final String message;
}
