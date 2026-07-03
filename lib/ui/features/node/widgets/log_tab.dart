import 'package:flutter/material.dart';

import '../../../../domain/models/link_log.dart';
import '../../../../domain/node_link_state.dart';
import '../../../core/app_theme.dart';

/// Jurnal Data Link — jejak event kanal (rute, ACK, retry, error server)
/// terbaru di atas, dibatasi 300 entri oleh mesin.
class LogTab extends StatelessWidget {
  const LogTab({super.key, required this.state});

  final NodeLinkState state;

  @override
  Widget build(BuildContext context) {
    final logs = state.logs;
    if (logs.isEmpty) {
      return const Center(
        child: Text('Belum ada aktivitas.',
            style: TextStyle(color: AppColors.textMuted, fontSize: 13)),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 24),
      itemCount: logs.length,
      itemBuilder: (context, index) {
        final log = logs[index];
        final color = switch (log.level) {
          LogLevel.ok => AppColors.emerald,
          LogLevel.warn => AppColors.amber,
          LogLevel.error => AppColors.rose,
          LogLevel.info => AppColors.textMuted,
        };
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: 4),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                _hhmmss(log.at),
                style: const TextStyle(
                  fontSize: 11,
                  color: AppColors.textMuted,
                  fontFeatures: [FontFeature.tabularFigures()],
                ),
              ),
              const SizedBox(width: 8),
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Container(
                  width: 7,
                  height: 7,
                  decoration:
                      BoxDecoration(color: color, shape: BoxShape.circle),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                child: Text(log.message,
                    style: TextStyle(
                        fontSize: 12.5,
                        color: AppColors.text.withValues(alpha: 0.92))),
              ),
            ],
          ),
        );
      },
    );
  }

  static String _hhmmss(DateTime t) =>
      '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}:${t.second.toString().padLeft(2, '0')}';
}
