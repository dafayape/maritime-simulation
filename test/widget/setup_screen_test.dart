import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/ui/features/setup/setup_screen.dart';
import 'package:maritim_node/ui/core/app_theme.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  group('SetupScreen', () {
    testWidgets('memuat preferensi dan merender formulir lengkap',
        (tester) async {
      SharedPreferences.setMockInitialValues({});
      await tester.pumpWidget(ProviderScope(
        child: MaterialApp(theme: buildAppTheme(), home: const SetupScreen()),
      ));
      await tester.pumpAndSettle();

      expect(find.text('Maritim Node'), findsOneWidget);
      expect(find.text('Alamat backend'), findsOneWidget);
      expect(find.text('Tes koneksi & muat sesi'), findsOneWidget);
      expect(find.text('Node ID kapal'), findsOneWidget);
      expect(find.text('Gunakan GPS asli perangkat'), findsOneWidget);

      // Node ID default terisi otomatis pola KPL-xxx.
      final nodeField = tester.widget<TextField>(
          find.widgetWithText(TextField, 'Node ID kapal'));
      expect(nodeField.controller!.text, startsWith('KPL-'));

      // Tombol hubungkan berada di dasar ListView lazy — gulir dahulu.
      await tester.scrollUntilVisible(find.byType(FilledButton), 200,
          scrollable: find.byType(Scrollable).first);

      // Belum ada sesi terpilih → tombol hubungkan terkunci.
      final connect = tester.widget<FilledButton>(find.byType(FilledButton));
      expect(connect.onPressed, isNull);
    });

    testWidgets('mode manual menampilkan lat/lng + drift, GPS asli tidak',
        (tester) async {
      SharedPreferences.setMockInitialValues({});
      await tester.pumpWidget(ProviderScope(
        child: MaterialApp(theme: buildAppTheme(), home: const SetupScreen()),
      ));
      await tester.pumpAndSettle();

      expect(find.text('Latitude'), findsOneWidget);
      expect(find.text('Longitude'), findsOneWidget);
      expect(find.textContaining('drift'), findsOneWidget);

      await tester.tap(find.text('Gunakan GPS asli perangkat'));
      await tester.pumpAndSettle();

      expect(find.text('Latitude'), findsNothing);
      expect(find.text('Longitude'), findsNothing);
    });
  });
}
