import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/domain/models/env_params.dart';
import 'package:maritim_node/domain/models/routing_info.dart';
import 'package:maritim_node/domain/node_link_state.dart';
import 'package:maritim_node/ui/core/app_theme.dart';
import 'package:maritim_node/ui/features/node/widgets/hud_panel.dart';

Widget _host(NodeLinkState state) => MaterialApp(
      theme: buildAppTheme(),
      home: Scaffold(body: HudPanel(state: state)),
    );

void main() {
  group('HudPanel', () {
    testWidgets('online + routed menampilkan parent, SF, dan penghitung',
        (tester) async {
      const state = NodeLinkState(
        phase: LinkPhase.online,
        nodeId: 'KPL-100',
        route: RoutingInfo(
          parentTarget: 'EDGE-PRATU-01',
          parentIsEdge: true,
          distanceToParentKm: 2.34,
          hopLevel: 1,
          status: 'routed',
        ),
        env: EnvParams(
          spreadingFactor: 10,
          maxPayloadBytes: 51,
          ackTimeoutMs: 6200,
          maxRetries: 3,
          txPowerDbm: 14,
          maxRangeKm: 4.9,
          weatherSeverity: 1,
        ),
        counters: LinkCounters(sent: 7, acked: 6, retried: 1),
      );
      await tester.pumpWidget(_host(state));

      expect(find.text('ONLINE'), findsOneWidget);
      expect(find.textContaining('EDGE-PRATU-01'), findsOneWidget);
      expect(find.textContaining('Syahbandar'), findsOneWidget);
      expect(find.textContaining('SF10'), findsOneWidget);
      expect(find.textContaining('≤51 B'), findsOneWidget);
      expect(find.text('7'), findsOneWidget); // kirim
      expect(find.text('6'), findsOneWidget); // ack
    });

    testWidgets('terisolasi + reconnecting ditampilkan dengan jelas',
        (tester) async {
      const state = NodeLinkState(
        phase: LinkPhase.reconnecting,
        reconnectAttempt: 2,
        reconnectIn: Duration(seconds: 4),
        route: RoutingInfo.isolated(),
      );
      await tester.pumpWidget(_host(state));

      expect(find.textContaining('RECONNECT #2'), findsOneWidget);
      expect(find.textContaining('TERISOLASI'), findsOneWidget);
    });

    testWidgets('banner sesi berakhir muncul dengan alasan', (tester) async {
      const state = NodeLinkState(
        phase: LinkPhase.ended,
        endedReason: 'Simulasi dihentikan dari dashboard.',
      );
      await tester.pumpWidget(_host(state));

      expect(find.text('SESI BERAKHIR'), findsOneWidget);
      expect(find.textContaining('dihentikan dari dashboard'), findsOneWidget);
    });
  });
}
