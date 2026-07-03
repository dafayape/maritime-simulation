import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../di/providers.dart';
import '../../../../domain/node_link_state.dart';

/// ViewModel perintah Kirim: mengisolasi pemanggilan asinkron mesin
/// data-link dari `onPressed` widget (Anti-Slop §1) dan memaparkan status
/// sibuk + hasil terakhir untuk umpan balik UI.
class TransmitViewModel extends AutoDisposeAsyncNotifier<TransmitOutcome?> {
  @override
  Future<TransmitOutcome?> build() async => null;

  Future<void> submit(Map<String, dynamic> values) async {
    if (state.isLoading) return;
    state = const AsyncLoading();
    final outcome = await ref
        .read(nodeLinkRepositoryProvider)
        .transmitUserPayload(values);
    state = AsyncData(outcome);
  }
}

final transmitViewModelProvider =
    AutoDisposeAsyncNotifierProvider<TransmitViewModel, TransmitOutcome?>(
        TransmitViewModel.new);
