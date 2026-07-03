import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/config/app_config.dart';
import '../../../../core/config/node_session_config.dart';
import '../../../../data/services/rest_client.dart';
import '../../../../di/providers.dart';
import '../../../../domain/models/session_summary.dart';

class SetupState {
  const SetupState({
    required this.config,
    this.sessions = const [],
    this.selectedSessionId,
    this.probing = false,
    this.connecting = false,
    this.serverOk = false,
    this.message,
    this.messageIsError = false,
  });

  final AppConfig config;
  final List<SessionSummary> sessions;
  final String? selectedSessionId;
  final bool probing;
  final bool connecting;
  final bool serverOk;
  final String? message;
  final bool messageIsError;

  SessionSummary? get selectedSession {
    for (final s in sessions) {
      if (s.id == selectedSessionId) return s;
    }
    return null;
  }

  SetupState copyWith({
    AppConfig? config,
    List<SessionSummary>? sessions,
    String? selectedSessionId,
    bool clearSelected = false,
    bool? probing,
    bool? connecting,
    bool? serverOk,
    String? message,
    bool clearMessage = false,
    bool? messageIsError,
  }) =>
      SetupState(
        config: config ?? this.config,
        sessions: sessions ?? this.sessions,
        selectedSessionId: clearSelected
            ? null
            : (selectedSessionId ?? this.selectedSessionId),
        probing: probing ?? this.probing,
        connecting: connecting ?? this.connecting,
        serverOk: serverOk ?? this.serverOk,
        message: clearMessage ? null : (message ?? this.message),
        messageIsError: messageIsError ?? this.messageIsError,
      );
}

/// ViewModel layar Setup: memuat/menyimpan preferensi, menguji server,
/// mengambil daftar sesi aktif, dan memulai mesin data-link. Seluruh
/// operasi asinkron hidup di sini — bukan di widget tree.
class SetupViewModel extends AutoDisposeAsyncNotifier<SetupState> {
  @override
  Future<SetupState> build() async {
    final config = await ref.watch(configRepositoryProvider).load();
    return SetupState(config: config);
  }

  void _emit(SetupState next) => state = AsyncData(next);

  SetupState? get _current => state.valueOrNull;

  /// Uji `/healthz` lalu muat daftar sesi aktif dari URL yang diberikan.
  Future<void> probeServer(String rawUrl) async {
    final current = _current;
    if (current == null || current.probing) return;

    final url = RestClient.normalizeBaseUrl(rawUrl);
    if (url.isEmpty) {
      _emit(current.copyWith(
          message: 'Isi alamat server terlebih dahulu.', messageIsError: true));
      return;
    }

    _emit(current.copyWith(
        probing: true, clearMessage: true, serverOk: false));
    final rest = RestClient(url);
    try {
      await rest.health();
      final sessions = await rest.listSessions();
      final active = sessions.where((s) => s.isActive).toList();

      String? selected;
      if (active.isNotEmpty) {
        final remembered = current.config.lastSessionId;
        selected = active.any((s) => s.id == remembered)
            ? remembered
            : active.first.id;
      }

      _emit((_current ?? current).copyWith(
        config: current.config.copyWith(serverUrl: url),
        sessions: active,
        selectedSessionId: selected,
        clearSelected: selected == null,
        probing: false,
        serverOk: true,
        message: active.isEmpty
            ? 'Server sehat, tetapi belum ada sesi aktif. Buat sesi dari '
                'dashboard web terlebih dahulu.'
            : 'Server sehat — ${active.length} sesi aktif ditemukan.',
        messageIsError: active.isEmpty,
      ));
    } on Object catch (e) {
      _emit((_current ?? current).copyWith(
        probing: false,
        serverOk: false,
        sessions: const [],
        clearSelected: true,
        message: e is RestException ? e.message : 'Gagal menghubungi server: $e',
        messageIsError: true,
      ));
    } finally {
      rest.dispose();
    }
  }

  void selectSession(String? id) {
    final current = _current;
    if (current == null) return;
    _emit(current.copyWith(
        selectedSessionId: id, clearSelected: id == null));
  }

  /// Validasi masukan, simpan preferensi, lalu nyalakan mesin data-link.
  /// Mengembalikan null bila sukses, atau pesan kesalahan untuk ditampilkan.
  Future<String?> connect({
    required String serverUrl,
    required String nodeId,
    required bool useRealGps,
    required double manualLat,
    required double manualLng,
    required bool driftEnabled,
  }) async {
    final current = _current;
    if (current == null || current.connecting) return null;

    final url = RestClient.normalizeBaseUrl(serverUrl);
    if (url.isEmpty) return 'Alamat server wajib diisi.';
    if (!nodeIdPattern.hasMatch(nodeId)) {
      return 'Node ID hanya boleh huruf/angka/titik/strip/garis bawah '
          '(1–100 karakter).';
    }
    final session = current.selectedSession;
    if (session == null) {
      return 'Pilih sesi simulasi aktif terlebih dahulu.';
    }
    if (manualLat < -90 || manualLat > 90) {
      return 'Latitude harus di antara -90 dan 90.';
    }
    if (manualLng < -180 || manualLng > 180) {
      return 'Longitude harus di antara -180 dan 180.';
    }

    _emit(current.copyWith(connecting: true, clearMessage: true));

    final newConfig = current.config.copyWith(
      serverUrl: url,
      nodeId: nodeId,
      useRealGps: useRealGps,
      manualLat: manualLat,
      manualLng: manualLng,
      driftEnabled: driftEnabled,
      lastSessionId: session.id,
    );
    await ref.read(configRepositoryProvider).save(newConfig);

    try {
      await ref.read(nodeLinkRepositoryProvider).start(NodeSessionConfig(
            baseUrl: url,
            sessionId: session.id,
            sessionName: session.sessionName,
            nodeId: nodeId,
            useRealGps: useRealGps,
            manualLat: manualLat,
            manualLng: manualLng,
            driftEnabled: driftEnabled,
          ));
      _emit((_current ?? current).copyWith(
          config: newConfig, connecting: false));
      return null;
    } on Object catch (e) {
      _emit((_current ?? current).copyWith(
        config: newConfig,
        connecting: false,
        message: 'Gagal memulai koneksi: $e',
        messageIsError: true,
      ));
      return 'Gagal memulai koneksi: $e';
    }
  }
}

final setupViewModelProvider =
    AutoDisposeAsyncNotifierProvider<SetupViewModel, SetupState>(
        SetupViewModel.new);
