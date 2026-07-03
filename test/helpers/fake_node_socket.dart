import 'dart:async';

import 'package:maritim_node/data/services/node_socket.dart';
import 'package:maritim_node/domain/node_event.dart';

/// Socket palsu untuk menguji mesin data-link tanpa jaringan: merekam semua
/// frame keluar dan membiarkan test menyuntikkan event masuk kapan saja.
class FakeNodeSocket implements NodeSocket {
  final _controller = StreamController<NodeEvent>.broadcast();

  /// Semua frame yang "mengudara", urut kronologis.
  final List<Map<String, dynamic>> sentFrames = [];

  bool connected = false;

  /// Bila true, [send] berpura-pura kanal putus (return false).
  bool failSend = false;

  String? lastBaseUrl;
  String? lastSessionId;
  String? lastNodeId;

  @override
  Stream<NodeEvent> get events => _controller.stream;

  @override
  bool get isConnected => connected;

  @override
  Future<void> connect({
    required String baseUrl,
    required String sessionId,
    required String nodeId,
  }) async {
    lastBaseUrl = baseUrl;
    lastSessionId = sessionId;
    lastNodeId = nodeId;
    connected = true;
    emit(const SocketUp());
  }

  @override
  bool send(Map<String, dynamic> frame) {
    if (failSend || !connected) return false;
    sentFrames.add(frame);
    return true;
  }

  @override
  Future<void> close() async {
    connected = false;
  }

  void emit(NodeEvent event) {
    if (!_controller.isClosed) _controller.add(event);
  }

  List<Map<String, dynamic>> framesOf(String eventName) =>
      sentFrames.where((f) => f['event'] == eventName).toList();
}
