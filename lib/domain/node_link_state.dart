/// Potret keadaan mesin data-link yang dipaparkan ke UI — immutable penuh
/// agar aman dikonsumsi Riverpod tanpa kejutan mutasi (SRS §5.1).
library;

import 'models/env_params.dart';
import 'models/geo.dart';
import 'models/link_log.dart';
import 'models/queue_entry.dart';
import 'models/routing_info.dart';
import 'models/schema_field.dart';

enum LinkPhase {
  /// Belum tersambung ke sesi mana pun.
  idle,

  /// Sedang membuka kanal pertama kali.
  connecting,

  /// Kanal radio hidup.
  online,

  /// Kanal putus; auto-reconnect exponential backoff sedang berjalan.
  reconnecting,

  /// Sesi dihentikan server — tidak ada reconnect.
  ended,
}

/// Penghitung statistik Data Link (padanan `fleetStats` pada nodesim).
class LinkCounters {
  const LinkCounters({
    this.sent = 0,
    this.acked = 0,
    this.retried = 0,
    this.failed = 0,
    this.relayed = 0,
    this.rfReceived = 0,
    this.duplicates = 0,
    this.pings = 0,
  });

  final int sent;
  final int acked;
  final int retried;
  final int failed;
  final int relayed;
  final int rfReceived;
  final int duplicates;
  final int pings;

  LinkCounters copyWith({
    int? sent,
    int? acked,
    int? retried,
    int? failed,
    int? relayed,
    int? rfReceived,
    int? duplicates,
    int? pings,
  }) =>
      LinkCounters(
        sent: sent ?? this.sent,
        acked: acked ?? this.acked,
        retried: retried ?? this.retried,
        failed: failed ?? this.failed,
        relayed: relayed ?? this.relayed,
        rfReceived: rfReceived ?? this.rfReceived,
        duplicates: duplicates ?? this.duplicates,
        pings: pings ?? this.pings,
      );
}

class NodeLinkState {
  const NodeLinkState({
    this.phase = LinkPhase.idle,
    this.reconnectAttempt = 0,
    this.reconnectIn,
    this.nodeId = '',
    this.sessionId = '',
    this.sessionName = '',
    this.env,
    this.schema = const [],
    this.estimatedPackedBytes = 0,
    this.route = const RoutingInfo.isolated(),
    this.queue = const [],
    this.logs = const [],
    this.counters = const LinkCounters(),
    this.position,
    this.gpsMode = GpsMode.manual,
    this.endedReason,
  });

  final LinkPhase phase;
  final int reconnectAttempt;
  final Duration? reconnectIn;
  final String nodeId;
  final String sessionId;
  final String sessionName;
  final EnvParams? env;
  final List<SchemaField> schema;
  final int estimatedPackedBytes;
  final RoutingInfo route;
  final List<QueueEntry> queue;
  final List<LinkLog> logs;
  final LinkCounters counters;
  final GeoPoint? position;
  final GpsMode gpsMode;
  final String? endedReason;

  bool get isOnline => phase == LinkPhase.online;

  /// Tombol Kirim boleh aktif hanya bila kanal hidup DAN rute tersedia
  /// (PRD §4.1 — terkunci saat isolated).
  bool get canTransmit => isOnline && route.isRouted;

  int get pendingCount =>
      queue.where((e) => e.status == QueueStatus.pending).length;

  NodeLinkState copyWith({
    LinkPhase? phase,
    int? reconnectAttempt,
    Duration? reconnectIn,
    bool clearReconnectIn = false,
    String? nodeId,
    String? sessionId,
    String? sessionName,
    EnvParams? env,
    List<SchemaField>? schema,
    int? estimatedPackedBytes,
    RoutingInfo? route,
    List<QueueEntry>? queue,
    List<LinkLog>? logs,
    LinkCounters? counters,
    GeoPoint? position,
    GpsMode? gpsMode,
    String? endedReason,
  }) =>
      NodeLinkState(
        phase: phase ?? this.phase,
        reconnectAttempt: reconnectAttempt ?? this.reconnectAttempt,
        reconnectIn:
            clearReconnectIn ? null : (reconnectIn ?? this.reconnectIn),
        nodeId: nodeId ?? this.nodeId,
        sessionId: sessionId ?? this.sessionId,
        sessionName: sessionName ?? this.sessionName,
        env: env ?? this.env,
        schema: schema ?? this.schema,
        estimatedPackedBytes: estimatedPackedBytes ?? this.estimatedPackedBytes,
        route: route ?? this.route,
        queue: queue ?? this.queue,
        logs: logs ?? this.logs,
        counters: counters ?? this.counters,
        position: position ?? this.position,
        gpsMode: gpsMode ?? this.gpsMode,
        endedReason: endedReason ?? this.endedReason,
      );
}

/// Hasil perintah transmit yang dilaporkan balik ke UI.
class TransmitOutcome {
  const TransmitOutcome._(this.accepted, this.message);

  const TransmitOutcome.ok(String message) : this._(true, message);

  const TransmitOutcome.rejected(String message) : this._(false, message);

  final bool accepted;
  final String message;
}
