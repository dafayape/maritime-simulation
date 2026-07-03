import 'dart:async';
import 'dart:convert';

import 'package:web_socket_channel/web_socket_channel.dart';

import '../../core/utils/backoff.dart';
import '../../domain/models/env_params.dart';
import '../../domain/models/mesh_ack.dart';
import '../../domain/models/routing_info.dart';
import '../../domain/models/schema_field.dart';
import '../../domain/models/transmit_frame.dart';
import '../../domain/node_event.dart';
import 'rest_client.dart';

/// Kanal radio virtual. Abstraksi ini memisahkan mesin data-link dari
/// transport nyata sehingga unit test dapat menyuntikkan socket palsu.
abstract class NodeSocket {
  Stream<NodeEvent> get events;

  bool get isConnected;

  Future<void> connect({
    required String baseUrl,
    required String sessionId,
    required String nodeId,
  });

  /// Mengirim satu frame JSON. `false` bila kanal sedang putus — pemanggil
  /// membiarkan paket tetap PENDING di SQLite (Graceful Disconnect, SRS §5.3).
  bool send(Map<String, dynamic> frame);

  Future<void> close();
}

/// Implementasi produksi di atas `web_socket_channel` dengan auto-reconnect
/// exponential backoff (1s→2s→4s→8s→16s→30s). `session:ended` mematikan
/// reconnect secara permanen; penutupan oleh pengguna juga tidak di-retry.
class WebSocketNodeSocket implements NodeSocket {
  WebSocketNodeSocket();

  final _events = StreamController<NodeEvent>.broadcast();
  final _backoff = ExponentialBackoff();

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  Timer? _retryTimer;

  String _baseUrl = '';
  String _sessionId = '';
  String _nodeId = '';
  bool _connected = false;
  bool _userClosed = false;
  bool _sessionEnded = false;

  @override
  Stream<NodeEvent> get events => _events.stream;

  @override
  bool get isConnected => _connected;

  @override
  Future<void> connect({
    required String baseUrl,
    required String sessionId,
    required String nodeId,
  }) async {
    _baseUrl = baseUrl;
    _sessionId = sessionId;
    _nodeId = nodeId;
    _userClosed = false;
    _sessionEnded = false;
    _backoff.reset();
    await _dial();
  }

  Future<void> _dial() async {
    if (_userClosed || _sessionEnded) return;

    final uri = RestClient.wsNodesUri(_baseUrl, _sessionId, _nodeId);
    try {
      final channel = WebSocketChannel.connect(uri);
      await channel.ready;
      _channel = channel;
      _connected = true;
      _backoff.reset();
      _emit(const SocketUp());

      _sub = channel.stream.listen(
        _onRaw,
        onDone: () => _onDown('koneksi ditutup server'),
        onError: (Object e) => _onDown('$e'),
        cancelOnError: true,
      );
    } on Object catch (e) {
      _onDown('gagal menyambung: $e');
    }
  }

  void _onRaw(dynamic raw) {
    Map<String, dynamic> json;
    try {
      json = jsonDecode(raw as String) as Map<String, dynamic>;
    } on Object {
      return; // frame korup diabaikan — jangan pernah crash (SRS §3 error handling)
    }

    switch (json['event']) {
      case 'env:sync_params':
        _emit(EnvUpdated(EnvParams.fromJson(json)));
      case 'schema:sync':
        _emit(SchemaUpdated(
          [
            for (final f in (json['fields'] as List<dynamic>? ?? []))
              SchemaField.fromJson(f as Map<String, dynamic>),
          ],
          (json['estimated_packed_bytes'] as num?)?.toInt() ?? 0,
        ));
      case 'mesh:routing_update':
        _emit(RouteUpdated(RoutingInfo.fromJson(json)));
      case 'mesh:receive_rf':
        _emit(RfReceived(TransmitFrame.fromJson(json)));
      case 'mesh:ack':
        _emit(AckReceived(MeshAck.fromJson(json)));
      case 'session:ended':
        _sessionEnded = true;
        _emit(SessionEndedEvent(
          json['message'] as String? ?? 'Simulasi dihentikan server.',
        ));
        unawaited(_teardownChannel());
      case 'error':
        _emit(ServerErrorEvent(
          json['code'] as String? ?? 'unknown',
          json['message'] as String? ?? '',
        ));
      default:
        break; // event monitor/tak dikenal — bukan urusan kanal node
    }
  }

  void _onDown(String reason) {
    if (!_connected && _retryTimer != null) return; // sudah dijadwalkan
    _connected = false;
    _sub?.cancel();
    _sub = null;
    _channel = null;

    if (_userClosed || _sessionEnded) {
      _emit(SocketDown(reason, null, _backoff.attempt));
      return;
    }
    final delay = _backoff.next();
    _emit(SocketDown(reason, delay, _backoff.attempt));
    _retryTimer = Timer(delay, () {
      _retryTimer = null;
      _dial();
    });
  }

  @override
  bool send(Map<String, dynamic> frame) {
    final channel = _channel;
    if (channel == null || !_connected) return false;
    try {
      channel.sink.add(jsonEncode(frame));
      return true;
    } on Object {
      return false;
    }
  }

  Future<void> _teardownChannel() async {
    _connected = false;
    await _sub?.cancel();
    _sub = null;
    try {
      await _channel?.sink.close(1000, 'selesai');
    } on Object {
      // kanal mungkin sudah mati — aman diabaikan
    }
    _channel = null;
  }

  @override
  Future<void> close() async {
    _userClosed = true;
    _retryTimer?.cancel();
    _retryTimer = null;
    await _teardownChannel();
    await _events.close();
  }

  void _emit(NodeEvent event) {
    if (!_events.isClosed) _events.add(event);
  }
}
