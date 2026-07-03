import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../di/providers.dart';
import '../../core/app_logo.dart';
import '../../core/app_theme.dart';
import 'widgets/hud_panel.dart';
import 'widgets/log_tab.dart';
import 'widgets/queue_tab.dart';
import 'widgets/transmit_tab.dart';

/// Layar utama node: HUD + tab Kirim / Antrean / Log. Seluruh keadaan
/// berasal dari aliran mesin data-link; layar ini murni presentasi.
class NodeScreen extends ConsumerWidget {
  const NodeScreen({super.key});

  Future<void> _confirmDisconnect(BuildContext context, WidgetRef ref) async {
    final navigator = Navigator.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Putuskan koneksi?'),
        content: const Text(
          'Kanal radio akan ditutup. Paket PENDING tetap tersimpan di '
          'antrean SQLite dan dipompa ulang saat tersambung kembali.',
          style: TextStyle(fontSize: 13.5, color: AppColors.textMuted),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            style: FilledButton.styleFrom(
                backgroundColor: AppColors.rose,
                minimumSize: const Size(0, 40)),
            child: const Text('Putuskan'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await ref.read(nodeLinkRepositoryProvider).stop();
    if (navigator.mounted) navigator.pop();
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final async = ref.watch(nodeLinkStateProvider);
    final state =
        async.valueOrNull ?? ref.read(nodeLinkRepositoryProvider).state;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) _confirmDisconnect(context, ref);
      },
      child: DefaultTabController(
        length: 3,
        child: Scaffold(
          appBar: AppBar(
            title: Row(
              children: [
                const AppLogo(size: 30),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(state.nodeId.isEmpty ? 'Node' : state.nodeId,
                          style: const TextStyle(
                              fontSize: 16, fontWeight: FontWeight.w700)),
                      if (state.sessionName.isNotEmpty)
                        Text(
                          state.sessionName,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                              fontSize: 11, color: AppColors.textMuted),
                        ),
                    ],
                  ),
                ),
              ],
            ),
            actions: [
              IconButton(
                tooltip: 'Putuskan koneksi',
                onPressed: () => _confirmDisconnect(context, ref),
                icon: const Icon(Icons.power_settings_new,
                    color: AppColors.rose),
              ),
            ],
          ),
          body: SafeArea(
            child: Column(
              children: [
                HudPanel(state: state),
                TabBar(
                  tabs: [
                    const Tab(text: 'Kirim'),
                    Tab(
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          const Text('Antrean'),
                          if (state.pendingCount > 0) ...[
                            const SizedBox(width: 6),
                            Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 6, vertical: 1),
                              decoration: BoxDecoration(
                                color:
                                    AppColors.amber.withValues(alpha: 0.18),
                                borderRadius: BorderRadius.circular(999),
                              ),
                              child: Text(
                                '${state.pendingCount}',
                                style: const TextStyle(
                                    fontSize: 10.5, color: AppColors.amber),
                              ),
                            ),
                          ],
                        ],
                      ),
                    ),
                    const Tab(text: 'Log'),
                  ],
                ),
                Expanded(
                  child: TabBarView(
                    children: [
                      TransmitTab(state: state),
                      QueueTab(state: state),
                      LogTab(state: state),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
