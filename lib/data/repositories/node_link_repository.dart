import 'dart:async';

import 'package:uuid/uuid.dart';

import '../../core/config/node_session_config.dart';
import '../../domain/models/link_log.dart';
import '../../domain/models/mesh_ack.dart';
import '../../domain/models/queue_entry.dart';
import '../../domain/models/routing_info.dart';
import '../../domain/models/transmit_frame.dart';
import '../../domain/node_event.dart';
import '../../domain/node_link_state.dart';
import '../services/location_service.dart';
import '../services/node_socket.dart';
import '../services/payload_codec.dart';
import '../services/queue_database.dart';
import '../services/rest_client.dart';

/// Mesin Data Link node (SRS §3) — SATU-SATUNYA tempat logika ACK/Timer,
/// antrean SQLite, dan relay hidup. Widget tree tidak pernah menyentuh
/// timer atau database secara langsung (Anti-Slop §1).
///
/// Siklus utama:
///  1. `start()` membuka kanal `/ws/nodes` + memulai ping GPS 3 detik;
///  2. transmit → simpan PENDING → tembak frame → `Timer` menunggu
///     `mesh:ack` selama `ack_timeout_ms` (+jeda toleransi 500 ms, sama
///     dengan harness `nodesim` backend);
///  3. timeout → transaksi `bumpRetryOrFail`: kirim ulang dengan
///     `packet_id` SAMA (inilah yang menguji dedup server) maksimal
///     `max_retries`, lalu FAILED (Link Lost);
///  4. `mesh:receive_rf` → balas `node:ack` seketika, dedup, simpan dengan
///     `origin_node` ASLI, teruskan ke parent node ini (hop+1);
///  5. rute pulih / kanal tersambung ulang → pompa semua PENDING.
class NodeLinkRepository {
  NodeLinkRepository({
    required this._socketFactory,
    required this._db,
    required this._location,
    this._restFactory,
    this.codec = const PayloadCodec(),
    Uuid? uuid,
    this.pingInterval = const Duration(seconds: 3),
    this.ackGrace = const Duration(milliseconds: 500),
  }) : _uuid = uuid ?? const Uuid();

  final NodeSocket Function() _socketFactory;
  final QueueDatabase _db;
  final LocationService _location;
  final RestClient Function(String baseUrl)? _restFactory;
  final PayloadCodec codec;
  final Uuid _uuid;
  final Duration pingInterval;
  final Duration ackGrace;

  static const _maxLogs = 300;

  /// Ambang fallback REST: bila dial gagal beruntun sebanyak ini, cek
  /// `is_active` sesi — event `session:ended` bisa kalah balapan dengan
  /// penutupan socket di server, jadi klien tidak boleh reconnect selamanya.
  static const _dialFailuresBeforeRestCheck = 3;

  final _states = StreamController<NodeLinkState>.broadcast();
  NodeLinkState _state = const NodeLinkState();

  NodeSocket? _socket;
  StreamSubscription<NodeEvent>? _eventSub;
  Timer? _pingTimer;
  final Map<String, Timer> _ackTimers = {};
  NodeSessionConfig? _cfg;
  RestClient? _rest;
  int _dialFailures = 0;
  bool _checkingActive = false;

  NodeLinkState get state => _state;

  /// Setiap pelanggan langsung menerima potret terkini, lalu pembaruan.
  Stream<NodeLinkState> get states async* {
    yield _state;
    yield* _states.stream;
  }

  // --- siklus hidup ---------------------------------------------------------

  Future<void> start(NodeSessionConfig cfg) async {
    await stop();
    _cfg = cfg;
    _rest = _restFactory?.call(cfg.baseUrl);
    _dialFailures = 0;

    _set((_) => NodeLinkState(
          phase: LinkPhase.connecting,
          nodeId: cfg.nodeId,
          sessionId: cfg.sessionId,
          sessionName: cfg.sessionName,
        ));

    if (cfg.useRealGps) {
      final gpsError = await _location.startRealGps();
      if (gpsError == null) {
        _set((s) => s.copyWith(gpsMode: _location.mode));
        _log(LogLevel.info, 'GPS asli aktif.');
      } else {
        _location.setManual(
          lat: cfg.manualLat,
          lng: cfg.manualLng,
          drift: cfg.driftEnabled,
        );
        _log(LogLevel.warn, '$gpsError Beralih ke koordinat manual.');
      }
    } else {
      _location.setManual(
        lat: cfg.manualLat,
        lng: cfg.manualLng,
        drift: cfg.driftEnabled,
      );
    }
    _set((s) => s.copyWith(
        gpsMode: _location.mode, position: _location.current));

    final socket = _socketFactory();
    _socket = socket;
    _eventSub = socket.events.listen(_onEvent);
    _log(LogLevel.info,
        'Menyambung ke ${cfg.baseUrl} sebagai ${cfg.nodeId}…');
    await socket.connect(
      baseUrl: cfg.baseUrl,
      sessionId: cfg.sessionId,
      nodeId: cfg.nodeId,
    );

    _pingTimer = Timer.periodic(pingInterval, (_) => _sendPing());
    await _refreshQueue();
  }

  /// Putus atas kehendak pengguna — antrean PENDING tetap tersimpan aman
  /// di SQLite untuk dipompa pada sesi berikutnya (Graceful Disconnect).
  Future<void> stop() async {
    _pingTimer?.cancel();
    _pingTimer = null;
    for (final t in _ackTimers.values) {
      t.cancel();
    }
    _ackTimers.clear();
    await _eventSub?.cancel();
    _eventSub = null;
    await _socket?.close();
    _socket = null;
    _rest?.dispose();
    _rest = null;
    await _location.dispose();
    if (_cfg != null) {
      _set((s) => s.copyWith(phase: LinkPhase.idle, clearReconnectIn: true));
    }
    _cfg = null;
  }

  Future<void> dispose() async {
    await stop();
    await _states.close();
  }

  // --- reaksi event kanal ---------------------------------------------------

  void _onEvent(NodeEvent event) {
    switch (event) {
      case SocketUp():
        _dialFailures = 0;
        _set((s) => s.copyWith(
            phase: LinkPhase.online,
            reconnectAttempt: 0,
            clearReconnectIn: true));
        _log(LogLevel.ok, 'Kanal radio tersambung.');
        _sendPing(); // daftarkan posisi segera (server menolak transmit tanpa posisi)

      case SocketDown(:final reason, :final willRetryIn, :final attempt):
        _onSocketDown(reason, willRetryIn, attempt);

      case EnvUpdated(:final params):
        _set((s) => s.copyWith(env: params));
        _log(
          LogLevel.info,
          'Parameter radio: SF${params.spreadingFactor} · limit '
          '${params.maxPayloadBytes} B · timeout ACK ${params.ackTimeoutMs} ms '
          '· retry maks ${params.maxRetries}.',
        );

      case SchemaUpdated(:final fields, :final estimatedPackedBytes):
        _set((s) => s.copyWith(
            schema: fields, estimatedPackedBytes: estimatedPackedBytes));
        _log(LogLevel.info,
            'Skema payload: ${fields.length} field (~$estimatedPackedBytes B).');

      case RouteUpdated(:final route):
        _onRouteUpdated(route);

      case RfReceived(:final frame):
        unawaited(_handleReceiveRf(frame));

      case AckReceived(:final ack):
        unawaited(_handleAck(ack));

      case SessionEndedEvent(:final message):
        _endSession(message);

      case ServerErrorEvent(:final code, :final message):
        _log(LogLevel.error, 'Server menolak frame [$code]: $message');
    }
  }

  void _onSocketDown(String reason, Duration? willRetryIn, int attempt) {
    // Radio mati: hentikan penantian ACK yang sedang berjalan; barisnya
    // tetap PENDING di SQLite dan akan dipompa ulang saat kanal pulih.
    for (final t in _ackTimers.values) {
      t.cancel();
    }
    _ackTimers.clear();

    if (_state.phase == LinkPhase.ended) return;
    if (willRetryIn == null) {
      _set((s) => s.copyWith(phase: LinkPhase.idle, clearReconnectIn: true));
      return;
    }

    _set((s) => s.copyWith(
        phase: LinkPhase.reconnecting,
        reconnectAttempt: attempt,
        reconnectIn: willRetryIn));
    _log(
      LogLevel.warn,
      'Kanal putus ($reason) — coba ulang ke-$attempt dalam '
      '${willRetryIn.inSeconds} s. Paket menunggu aman di antrean.',
    );

    if (reason.startsWith('gagal menyambung')) {
      _dialFailures++;
      if (_dialFailures >= _dialFailuresBeforeRestCheck) {
        unawaited(_verifySessionStillActive());
      }
    }
  }

  /// Fallback REST `is_active` — `session:ended` dari server dapat kalah
  /// balapan dengan penutupan socket, sehingga tanpa pengecekan ini klien
  /// akan mencoba menyambung ulang selamanya ke sesi yang sudah mati.
  Future<void> _verifySessionStillActive() async {
    final rest = _rest;
    final cfg = _cfg;
    if (rest == null || cfg == null || _checkingActive) return;
    _checkingActive = true;
    try {
      final session = await rest.getSession(cfg.sessionId);
      if (session == null || !session.isActive) {
        _endSession('Sesi sudah tidak aktif (verifikasi REST).');
      }
    } on Object {
      // Server tak terjangkau — biarkan backoff terus mencoba.
    } finally {
      _checkingActive = false;
    }
  }

  void _onRouteUpdated(RoutingInfo route) {
    final wasRouted = _state.route.isRouted;
    _set((s) => s.copyWith(route: route));
    if (route.isRouted) {
      _log(
        LogLevel.ok,
        'Rute baru: parent ${route.parentTarget}'
        '${route.parentIsEdge ? ' (Syahbandar)' : ''} · hop level '
        '${route.hopLevel} · ${route.distanceToParentKm.toStringAsFixed(2)} km.',
      );
      unawaited(_retargetAndPump(route.parentTarget));
    } else {
      if (wasRouted) {
        _log(LogLevel.warn,
            'TERISOLASI — rute hilang; transmisi dikunci, antrean ditahan.');
      } else {
        _log(LogLevel.warn, 'Belum ada rute (isolated).');
      }
    }
  }

  Future<void> _retargetAndPump(String newParent) async {
    final cfg = _cfg;
    if (cfg == null) return;
    await _db.retargetPending(cfg.sessionId, newParent);
    await _pumpPending();
  }

  void _endSession(String reason) {
    _pingTimer?.cancel();
    _pingTimer = null;
    for (final t in _ackTimers.values) {
      t.cancel();
    }
    _ackTimers.clear();
    _set((s) => s.copyWith(
        phase: LinkPhase.ended, endedReason: reason, clearReconnectIn: true));
    _log(LogLevel.warn, 'Sesi berakhir: $reason');
    unawaited(_socket?.close() ?? Future.value());
  }

  // --- ping GPS (PRD §5 node:ping) -----------------------------------------

  void _sendPing() {
    final cfg = _cfg;
    final socket = _socket;
    if (cfg == null || socket == null) return;

    final pos = _location.tick(pingInterval);
    _set((s) => s.copyWith(position: pos));
    final sent = socket.send({
      'event': 'node:ping',
      'node_id': cfg.nodeId,
      'session_id': cfg.sessionId,
      'lat': pos.lat,
      'lng': pos.lng,
    });
    if (sent) {
      _set((s) =>
          s.copyWith(counters: s.counters.copyWith(pings: s.counters.pings + 1)));
    }
  }

  // --- originasi payload pengguna (SRS §3A + §3B) ---------------------------

  /// Ukur payload form saat ini — angka yang sama persis dengan yang akan
  /// dikirim, karena memakai codec dan skema yang sama (Binary-First).
  PackOutcome measure(Map<String, dynamic> values) =>
      codec.pack(values, _state.schema);

  Future<TransmitOutcome> transmitUserPayload(
      Map<String, dynamic> values) async {
    final cfg = _cfg;
    final env = _state.env;
    if (cfg == null || _state.phase == LinkPhase.ended) {
      return const TransmitOutcome.rejected('Sesi sudah berakhir.');
    }
    if (env == null) {
      return const TransmitOutcome.rejected(
          'Parameter radio belum diterima dari server.');
    }

    final packed = codec.pack(values, _state.schema);
    switch (packed) {
      case PackFailure(:final message):
        return TransmitOutcome.rejected(message);
      case PackSuccess():
        if (packed.size > env.maxPayloadBytes) {
          // Pagar terakhir Binary-First — UI seharusnya sudah mengunci tombol.
          return TransmitOutcome.rejected(
            'Payload ${packed.size} B melampaui limit fisik LoRa '
            'SF${env.spreadingFactor} (${env.maxPayloadBytes} B).',
          );
        }

        final entry = QueueEntry(
          packetId: _uuid.v4(),
          originNodeId: cfg.nodeId,
          targetParent: _state.route.parentTarget,
          payloadB64: packed.base64Payload,
          status: QueueStatus.pending,
          retryCount: 0,
          hopCount: 1,
          routingPath: [cfg.nodeId],
        );
        await _db.insertPendingIfNew(cfg.sessionId, entry);
        await _refreshQueue();

        if (!_state.canTransmit) {
          _log(LogLevel.warn,
              'Paket ${_short(entry.packetId)} ditahan (${_state.isOnline ? 'terisolasi' : 'offline'}).');
          return const TransmitOutcome.ok(
              'Disimpan di antrean — menunggu rute/koneksi pulih.');
        }
        _fire(entry);
        return TransmitOutcome.ok(
            'Frame ${packed.size} B mengudara — menunggu ACK.');
    }
  }

  // --- relay mesh (SRS §3C) -------------------------------------------------

  Future<void> _handleReceiveRf(TransmitFrame frame) async {
    final cfg = _cfg;
    final socket = _socket;
    if (cfg == null || socket == null) return;

    // 1. ACK seketika ke pengirim — penerima nyata juga meng-ACK duplikat.
    socket.send({
      'event': 'node:ack',
      'packet_id': frame.packetId,
      'receiver_node': cfg.nodeId,
      'status': 'received',
    });
    _set((s) => s.copyWith(
        counters:
            s.counters.copyWith(rfReceived: s.counters.rfReceived + 1)));

    // 2+3. Simpan dengan origin_node ASLI (bukan identitas relay ini) lalu
    // teruskan ke parent. Keunikan packet_id = dedup.
    final relay = frame.relayVia(
      nodeId: cfg.nodeId,
      newParent: _state.route.parentTarget,
    );
    final entry = QueueEntry(
      packetId: relay.packetId,
      originNodeId: relay.originNode,
      targetParent: relay.targetParent,
      payloadB64: relay.binaryPayloadB64,
      status: QueueStatus.pending,
      retryCount: 0,
      hopCount: relay.hopCount,
      routingPath: relay.routingPath,
    );
    final isNew = await _db.insertPendingIfNew(cfg.sessionId, entry);
    if (!isNew) {
      _set((s) => s.copyWith(
          counters:
              s.counters.copyWith(duplicates: s.counters.duplicates + 1)));
      _log(LogLevel.info,
          'Duplikat ${_short(frame.packetId)} — di-ACK, tidak diteruskan ulang.');
      return;
    }
    await _refreshQueue();

    if (!_state.canTransmit) {
      _log(
        LogLevel.warn,
        'Relay ${_short(frame.packetId)} asal ${frame.originNode} ditahan — '
        'node ini sedang tanpa rute.',
      );
      return;
    }
    _set((s) => s.copyWith(
        counters: s.counters.copyWith(relayed: s.counters.relayed + 1)));
    _log(
      LogLevel.ok,
      'Relay ${_short(frame.packetId)} asal ${frame.originNode} → '
      '${entry.targetParent} (hop ${entry.hopCount}).',
    );
    _fire(entry);
  }

  // --- mesin ACK & auto-retry (SRS §3B) --------------------------------------

  /// Menembakkan satu frame dan memasang Timer penantian `mesh:ack`.
  void _fire(QueueEntry entry) {
    final socket = _socket;
    final env = _state.env;
    if (socket == null || env == null) return;

    final frame = TransmitFrame(
      packetId: entry.packetId,
      originNode: entry.originNodeId,
      targetParent: entry.targetParent,
      hopCount: entry.hopCount,
      routingPath: entry.routingPath,
      binaryPayloadB64: entry.payloadB64,
    );
    final sent = socket.send(frame.toTransmitJson());
    if (!sent) {
      _log(LogLevel.warn,
          'Kanal sedang putus — ${_short(entry.packetId)} tetap PENDING.');
      return;
    }
    _set((s) =>
        s.copyWith(counters: s.counters.copyWith(sent: s.counters.sent + 1)));

    _ackTimers[entry.packetId]?.cancel();
    _ackTimers[entry.packetId] = Timer(
      env.ackTimeout + ackGrace,
      () => unawaited(_onAckTimeout(entry.packetId)),
    );
  }

  Future<void> _onAckTimeout(String packetId) async {
    _ackTimers.remove(packetId);
    final env = _state.env;
    if (env == null || _state.phase == LinkPhase.ended) return;

    final nextRetry = await _db.bumpRetryOrFail(packetId, env.maxRetries);
    if (nextRetry == null) {
      final row = await _db.byPacketId(packetId);
      if (row != null && row.status == QueueStatus.failed) {
        _set((s) => s.copyWith(
            counters: s.counters.copyWith(failed: s.counters.failed + 1)));
        _log(
          LogLevel.error,
          'Link mati: ${_short(packetId)} FAILED setelah ${env.maxRetries} '
          'retry — menunggu rute baru.',
        );
        await _refreshQueue();
      }
      return; // sudah ACKED (ACK menang balapan) atau baris tak dikenal
    }

    _set((s) => s.copyWith(
        counters: s.counters.copyWith(retried: s.counters.retried + 1)));
    _log(LogLevel.warn,
        'Timeout ACK — retry ke-$nextRetry untuk ${_short(packetId)}.');
    await _refreshQueue();

    final row = await _db.byPacketId(packetId);
    if (row != null && row.status == QueueStatus.pending) {
      _fire(row); // packet_id SAMA — menguji dedup sisi server
    }
  }

  Future<void> _handleAck(MeshAck ack) async {
    final timer = _ackTimers.remove(ack.packetId);
    timer?.cancel();

    if (timer == null) {
      // ACK basi (paket sudah ACKED/FAILED) — cukup dicatat.
      _log(LogLevel.info,
          'ACK ${ack.duplicate ? 'duplikat ' : 'basi '}${_short(ack.packetId)} dari ${ack.receiverNode}.');
      return;
    }

    await _db.markAcked(ack.packetId);
    _set((s) =>
        s.copyWith(counters: s.counters.copyWith(acked: s.counters.acked + 1)));
    _log(LogLevel.ok,
        'ACK ${_short(ack.packetId)} diterima ${ack.receiverNode}.');
    await _refreshQueue();
  }

  // --- pompa antrean Store-and-Forward ---------------------------------------

  bool _pumping = false;

  /// Mengirim ulang seluruh paket PENDING yang tidak sedang menunggu ACK —
  /// dipanggil saat rute pulih atau kanal tersambung kembali.
  Future<void> _pumpPending() async {
    final cfg = _cfg;
    if (cfg == null || _pumping || !_state.canTransmit) return;
    _pumping = true;
    try {
      final rows = await _db.pendingEntries(cfg.sessionId);
      final waiting =
          rows.where((r) => !_ackTimers.containsKey(r.packetId)).toList();
      if (waiting.isEmpty) return;
      _log(LogLevel.info,
          'Memompa ${waiting.length} paket tertunda dari antrean.');
      for (final row in waiting) {
        if (!_state.canTransmit) break;
        _fire(row);
      }
    } finally {
      _pumping = false;
    }
  }

  // --- utilitas ---------------------------------------------------------------

  Future<void> _refreshQueue() async {
    final cfg = _cfg;
    if (cfg == null) return;
    final rows = await _db.recentEntries(cfg.sessionId);
    _set((s) => s.copyWith(queue: rows));
  }

  void _log(LogLevel level, String message) {
    _set((s) {
      final logs = [LinkLog(level, message), ...s.logs];
      return s.copyWith(
        logs: logs.length > _maxLogs ? logs.sublist(0, _maxLogs) : logs,
      );
    });
  }

  void _set(NodeLinkState Function(NodeLinkState) update) {
    _state = update(_state);
    if (!_states.isClosed) _states.add(_state);
  }

  static String _short(String packetId) =>
      packetId.length <= 8 ? packetId : packetId.substring(0, 8);
}
