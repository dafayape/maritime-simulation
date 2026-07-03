import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/data/repositories/node_link_repository.dart';
import 'package:maritim_node/data/services/location_service.dart';
import 'package:maritim_node/data/services/queue_database.dart';
import 'package:maritim_node/di/providers.dart';
import 'package:maritim_node/domain/models/env_params.dart';
import 'package:maritim_node/domain/models/routing_info.dart';
import 'package:maritim_node/domain/models/schema_field.dart';
import 'package:maritim_node/domain/node_link_state.dart';
import 'package:maritim_node/ui/core/app_theme.dart';
import 'package:maritim_node/ui/features/node/widgets/transmit_tab.dart';
import 'package:sqflite_common_ffi/sqflite_ffi.dart';

import '../helpers/fake_node_socket.dart';

const _routed = RoutingInfo(
  parentTarget: 'KPL-001',
  parentIsEdge: false,
  distanceToParentKm: 1,
  hopLevel: 2,
  status: 'routed',
);

EnvParams _env({int limit = 51}) => EnvParams(
      spreadingFactor: 10,
      maxPayloadBytes: limit,
      ackTimeoutMs: 6200,
      maxRetries: 3,
      txPowerDbm: 14,
      maxRangeKm: 4.9,
      weatherSeverity: 1,
    );

void main() {
  setUpAll(sqfliteFfiInit);

  late NodeLinkRepository repo;
  late QueueDatabase db;

  setUp(() {
    db = QueueDatabase(factory: databaseFactoryFfi, path: inMemoryDatabasePath);
    repo = NodeLinkRepository(
      socketFactory: FakeNodeSocket.new,
      db: db,
      location: LocationService(),
    );
  });

  tearDown(() async {
    await repo.dispose();
    await db.close();
  });

  Widget host(NodeLinkState state) => ProviderScope(
        overrides: [nodeLinkRepositoryProvider.overrideWithValue(repo)],
        child: MaterialApp(
          theme: buildAppTheme(),
          home: Scaffold(body: TransmitTab(state: state)),
        ),
      );

  group('TransmitTab — form dinamis + byte counter', () {
    testWidgets('field tersusun dari skema server dan meter byte hidup',
        (tester) async {
      final state = NodeLinkState(
        phase: LinkPhase.online,
        route: _routed,
        env: _env(),
        schema: const [
          SchemaField(name: 'berat_kg', type: 'uint16'),
          SchemaField(name: 'jenis_ikan', type: 'string_10'),
          SchemaField(name: 'fresh', type: 'bool'),
        ],
        estimatedPackedBytes: 17,
      );
      await tester.pumpWidget(host(state));

      expect(find.widgetWithText(TextField, 'berat_kg'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'jenis_ikan'), findsOneWidget);
      expect(find.text('fresh'), findsOneWidget); // switch bool

      await tester.enterText(
          find.widgetWithText(TextField, 'berat_kg'), '300');
      await tester.pump();

      expect(find.textContaining('/ 51 B (SF10)'), findsOneWidget);

      final button = tester.widget<FilledButton>(find.byType(FilledButton));
      expect(button.onPressed, isNotNull,
          reason: 'payload kecil + online + routed → tombol aktif');
    });

    testWidgets('payload melebihi limit SF mengunci tombol Kirim',
        (tester) async {
      final state = NodeLinkState(
        phase: LinkPhase.online,
        route: _routed,
        env: _env(limit: 12),
        schema: const [SchemaField(name: 'jenis_ikan', type: 'string_20')],
      );
      await tester.pumpWidget(host(state));

      await tester.enterText(
          find.widgetWithText(TextField, 'jenis_ikan'), 'cakalang-besar');
      await tester.pump();

      final button = tester.widget<FilledButton>(find.byType(FilledButton));
      expect(button.onPressed, isNull);
      expect(find.textContaining('melampaui limit fisik LoRa'), findsOneWidget);
    });

    testWidgets('terisolasi mengunci tombol dengan label alasan',
        (tester) async {
      final state = NodeLinkState(
        phase: LinkPhase.online,
        route: const RoutingInfo.isolated(),
        env: _env(),
        schema: const [SchemaField(name: 'berat_kg', type: 'uint16')],
      );
      await tester.pumpWidget(host(state));
      await tester.pump(); // selesaikan build AsyncNotifier ViewModel

      final button = tester.widget<FilledButton>(find.byType(FilledButton));
      expect(button.onPressed, isNull);
      expect(find.text('Terisolasi — tanpa rute'), findsOneWidget);
    });

    testWidgets('tanpa skema server memakai skema bawaan nodesim',
        (tester) async {
      final state = NodeLinkState(
        phase: LinkPhase.online,
        route: _routed,
        env: _env(),
      );
      await tester.pumpWidget(host(state));
      await tester.pump();

      // Judul SectionCard dirender kapital — cocokkan tanpa peduli huruf.
      expect(
        find.textContaining(RegExp('skema bawaan', caseSensitive: false)),
        findsOneWidget,
      );
      expect(find.widgetWithText(TextField, 'lat'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'lng'), findsOneWidget);
      expect(find.widgetWithText(TextField, 'berat_kg'), findsOneWidget);
    });
  });
}
