import 'package:flutter/material.dart';

import '../../../../domain/models/queue_entry.dart';
import '../../../../domain/node_link_state.dart';
import '../../../core/app_theme.dart';

/// Antrean Store-and-Forward: memvisualkan isi tabel `transmit_queue`
/// (PENDING/ACKED/FAILED, retry, hop, dan badge relay dengan origin asli).
class QueueTab extends StatelessWidget {
  const QueueTab({super.key, required this.state});

  final NodeLinkState state;

  @override
  Widget build(BuildContext context) {
    final entries = state.queue;
    if (entries.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(24),
          child: Text(
            'Antrean kosong.\nFrame yang dikirim atau di-relay akan tercatat '
            'di sini beserta status ACK-nya.',
            textAlign: TextAlign.center,
            style: TextStyle(color: AppColors.textMuted, fontSize: 13),
          ),
        ),
      );
    }

    return ListView.separated(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 24),
      itemCount: entries.length,
      separatorBuilder: (_, _) => const SizedBox(height: 8),
      itemBuilder: (context, index) =>
          _QueueTile(entry: entries[index], myNodeId: state.nodeId),
    );
  }
}

class _QueueTile extends StatelessWidget {
  const _QueueTile({required this.entry, required this.myNodeId});

  final QueueEntry entry;
  final String myNodeId;

  @override
  Widget build(BuildContext context) {
    final (icon, color, label) = switch (entry.status) {
      QueueStatus.pending => (Icons.hourglass_top, AppColors.amber, 'PENDING'),
      QueueStatus.acked => (Icons.check_circle, AppColors.emerald, 'ACKED'),
      QueueStatus.failed => (Icons.cancel, AppColors.rose, 'FAILED'),
    };
    final isRelay = entry.isRelayFor(myNodeId);
    final shortId = entry.packetId.length > 8
        ? entry.packetId.substring(0, 8)
        : entry.packetId;
    final time = entry.createdAt?.toLocal();

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: AppColors.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.surfaceHi),
      ),
      child: Row(
        children: [
          Icon(icon, size: 20, color: color),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text('pkt $shortId…',
                        style: const TextStyle(
                            fontSize: 13.5, fontWeight: FontWeight.w600)),
                    const SizedBox(width: 8),
                    if (isRelay)
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: AppColors.cyan.withValues(alpha: 0.12),
                          borderRadius: BorderRadius.circular(6),
                          border: Border.all(
                              color:
                                  AppColors.cyan.withValues(alpha: 0.4)),
                        ),
                        child: Text(
                          'RELAY · asal ${entry.originNodeId}',
                          style: const TextStyle(
                              fontSize: 10, color: AppColors.cyan),
                        ),
                      ),
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  '→ ${entry.targetParent.isEmpty ? '(menunggu rute)' : entry.targetParent}'
                  ' · hop ${entry.hopCount}'
                  ' · retry ${entry.retryCount}'
                  '${time != null ? ' · ${_hhmmss(time)}' : ''}',
                  style: const TextStyle(
                      fontSize: 11.5, color: AppColors.textMuted),
                ),
              ],
            ),
          ),
          Text(label,
              style: TextStyle(
                  fontSize: 11, fontWeight: FontWeight.w700, color: color)),
        ],
      ),
    );
  }

  static String _hhmmss(DateTime t) =>
      '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}:${t.second.toString().padLeft(2, '0')}';
}
