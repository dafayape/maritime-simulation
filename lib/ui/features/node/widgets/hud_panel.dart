import 'package:flutter/material.dart';

import '../../../../domain/models/geo.dart';
import '../../../../domain/node_link_state.dart';
import '../../../core/app_theme.dart';
import '../../../core/widgets.dart';

/// Heads-Up Display (PRD §4.1): status kanal, Next Hop Target, parameter
/// radio, posisi GPS, dan penghitung Data Link — selalu terlihat di atas tab.
class HudPanel extends StatelessWidget {
  const HudPanel({super.key, required this.state});

  final NodeLinkState state;

  @override
  Widget build(BuildContext context) {
    final env = state.env;
    final route = state.route;

    final (connLabel, connColor) = switch (state.phase) {
      LinkPhase.online => ('ONLINE', AppColors.emerald),
      LinkPhase.connecting => ('MENYAMBUNG…', AppColors.amber),
      LinkPhase.reconnecting => (
          'RECONNECT #${state.reconnectAttempt}'
              '${state.reconnectIn != null ? ' · ${state.reconnectIn!.inSeconds}s' : ''}',
          AppColors.amber
        ),
      LinkPhase.ended => ('SESI BERAKHIR', AppColors.rose),
      LinkPhase.idle => ('TERPUTUS', AppColors.rose),
    };

    return Padding(
      padding: const EdgeInsets.fromLTRB(14, 10, 14, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              StatusChip(label: connLabel, color: connColor),
              if (route.isRouted)
                StatusChip(
                  icon: route.parentIsEdge ? Icons.anchor : Icons.sailing,
                  label:
                      '→ ${route.parentTarget}${route.parentIsEdge ? ' · Syahbandar' : ''}'
                      ' · hop ${route.hopLevel}'
                      ' · ${route.distanceToParentKm.toStringAsFixed(1)} km',
                  color: route.parentIsEdge ? AppColors.cyan : AppColors.sky,
                )
              else
                const StatusChip(
                    icon: Icons.link_off,
                    label: 'TERISOLASI — tanpa rute',
                    color: AppColors.rose),
              if (env != null)
                StatusChip(
                  icon: Icons.settings_input_antenna,
                  label: 'SF${env.spreadingFactor} · ≤${env.maxPayloadBytes} B'
                      ' · ACK ${(env.ackTimeoutMs / 1000).toStringAsFixed(1)}s',
                  color: AppColors.sky,
                ),
              if (state.position != null)
                StatusChip(
                  icon: Icons.gps_fixed,
                  label:
                      '${state.position} · ${state.gpsMode == GpsMode.real ? 'GPS asli' : 'manual'}',
                  color: AppColors.textMuted,
                ),
            ],
          ),
          const SizedBox(height: 8),
          _CountersRow(counters: state.counters),
          if (state.endedReason != null) ...[
            const SizedBox(height: 8),
            Container(
              padding:
                  const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                color: AppColors.rose.withValues(alpha: 0.10),
                borderRadius: BorderRadius.circular(10),
                border:
                    Border.all(color: AppColors.rose.withValues(alpha: 0.4)),
              ),
              child: Row(
                children: [
                  const Icon(Icons.info_outline,
                      size: 16, color: AppColors.rose),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      state.endedReason!,
                      style: const TextStyle(
                          color: AppColors.rose, fontSize: 12.5),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _CountersRow extends StatelessWidget {
  const _CountersRow({required this.counters});

  final LinkCounters counters;

  @override
  Widget build(BuildContext context) {
    Widget item(String label, int value, Color color) => Padding(
          padding: const EdgeInsets.only(right: 14),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(label,
                  style: const TextStyle(
                      color: AppColors.textMuted, fontSize: 11)),
              const SizedBox(width: 4),
              TabularText('$value', color: color, size: 12),
            ],
          ),
        );

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          item('kirim', counters.sent, AppColors.text),
          item('ack', counters.acked, AppColors.emerald),
          item('retry', counters.retried, AppColors.amber),
          item('gagal', counters.failed, AppColors.rose),
          item('relay', counters.relayed, AppColors.cyan),
          item('rf masuk', counters.rfReceived, AppColors.sky),
          item('duplikat', counters.duplicates, AppColors.textMuted),
          item('ping', counters.pings, AppColors.textMuted),
        ],
      ),
    );
  }
}
