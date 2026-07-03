/// Graf dependensi Riverpod — semua Service/Repository dirakit di sini
/// sehingga mudah di-override pada widget test maupun uji integrasi.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/repositories/config_repository.dart';
import '../data/repositories/node_link_repository.dart';
import '../data/services/location_service.dart';
import '../data/services/node_socket.dart';
import '../data/services/queue_database.dart';
import '../data/services/rest_client.dart';
import '../domain/node_link_state.dart';

final configRepositoryProvider =
    Provider<ConfigRepository>((ref) => ConfigRepository());

final queueDatabaseProvider = Provider<QueueDatabase>((ref) {
  final db = QueueDatabase();
  ref.onDispose(db.close);
  return db;
});

final locationServiceProvider = Provider<LocationService>((ref) {
  final service = LocationService();
  ref.onDispose(service.dispose);
  return service;
});

final nodeLinkRepositoryProvider = Provider<NodeLinkRepository>((ref) {
  final repo = NodeLinkRepository(
    socketFactory: WebSocketNodeSocket.new,
    db: ref.watch(queueDatabaseProvider),
    location: ref.watch(locationServiceProvider),
    restFactory: RestClient.new,
  );
  ref.onDispose(repo.dispose);
  return repo;
});

/// Aliran potret keadaan mesin data-link untuk seluruh HUD.
final nodeLinkStateProvider = StreamProvider<NodeLinkState>(
  (ref) => ref.watch(nodeLinkRepositoryProvider).states,
);
