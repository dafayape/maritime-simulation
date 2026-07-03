/// Union tertutup semua event masuk dari kanal node (`/ws/nodes`), hasil
/// decode amplop JSON backend. Registry lengkap: backend
/// `internal/protocol/events.go`.
library;

import 'models/env_params.dart';
import 'models/mesh_ack.dart';
import 'models/routing_info.dart';
import 'models/schema_field.dart';
import 'models/transmit_frame.dart';

sealed class NodeEvent {
  const NodeEvent();
}

/// `env:sync_params` — parameter radio berubah (SF, limit byte, timeout ACK…).
class EnvUpdated extends NodeEvent {
  const EnvUpdated(this.params);

  final EnvParams params;
}

/// `schema:sync` — skema payload dinamis aktif didorong server.
class SchemaUpdated extends NodeEvent {
  const SchemaUpdated(this.fields, this.estimatedPackedBytes);

  final List<SchemaField> fields;
  final int estimatedPackedBytes;
}

/// `mesh:routing_update` — parent/next-hop node ini diperbarui.
class RouteUpdated extends NodeEvent {
  const RouteUpdated(this.route);

  final RoutingInfo route;
}

/// `mesh:receive_rf` — node ini ditunjuk menjadi relay untuk frame asing.
class RfReceived extends NodeEvent {
  const RfReceived(this.frame);

  final TransmitFrame frame;
}

/// `mesh:ack` — kaki balik ACK dari penerima menuju pemancar.
class AckReceived extends NodeEvent {
  const AckReceived(this.ack);

  final MeshAck ack;
}

/// `session:ended` — simulasi dihentikan dari dashboard.
class SessionEndedEvent extends NodeEvent {
  const SessionEndedEvent(this.message);

  final String message;
}

/// `error` — frame kita ditolak server (invalid_envelope, transmitter_mismatch…).
class ServerErrorEvent extends NodeEvent {
  const ServerErrorEvent(this.code, this.message);

  final String code;
  final String message;
}

/// Sinyal transport: kanal tersambung dan siap kirim.
class SocketUp extends NodeEvent {
  const SocketUp();
}

/// Sinyal transport: kanal putus; `willRetryIn` null berarti tidak akan
/// menyambung ulang (ditutup pengguna / sesi berakhir).
class SocketDown extends NodeEvent {
  const SocketDown(this.reason, this.willRetryIn, this.attempt);

  final String reason;
  final Duration? willRetryIn;
  final int attempt;
}
