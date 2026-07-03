import 'dart:math' as math;

import 'package:flutter/material.dart';

import 'app_theme.dart';

/// Logo mesh produk digambar vektor via [CustomPaint] — geometri identik
/// dengan frontend/src/app/icon.svg (dua node kapal emerald, node Syahbandar
/// cyan bertanda plus, link putus-putus sky, ombak navy) sehingga tajam di
/// segala ukuran tanpa aset bitmap.
class AppLogo extends StatelessWidget {
  const AppLogo({super.key, this.size = 96, this.withBackground = true});

  final double size;
  final bool withBackground;

  @override
  Widget build(BuildContext context) {
    return CustomPaint(
      size: Size.square(size),
      painter: _LogoPainter(withBackground: withBackground),
    );
  }
}

class _LogoPainter extends CustomPainter {
  const _LogoPainter({required this.withBackground});

  final bool withBackground;

  @override
  void paint(Canvas canvas, Size size) {
    final s = size.width / 64.0; // skala dari viewBox SVG 64x64

    if (withBackground) {
      final bg = Paint()..color = AppColors.surface;
      canvas.drawRRect(
        RRect.fromRectAndRadius(Offset.zero & size, Radius.circular(14 * s)),
        bg,
      );
    }

    // Ombak: M8 44 c6-6 10-2 16-6
    final wave = Paint()
      ..color = AppColors.navy
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2 * s
      ..strokeCap = StrokeCap.round;
    final wavePath = Path()
      ..moveTo(8 * s, 44 * s)
      ..cubicTo(14 * s, 38 * s, 18 * s, 42 * s, 24 * s, 38 * s);
    canvas.drawPath(wavePath, wave);

    // Link mesh putus-putus (dasharray 4 3, lebar 2.5).
    final link = Paint()
      ..color = AppColors.sky
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2.5 * s;
    _dashedLine(
        canvas, Offset(18 * s, 46 * s), Offset(32 * s, 20 * s), 4 * s, 3 * s, link);
    _dashedLine(
        canvas, Offset(32 * s, 20 * s), Offset(48 * s, 42 * s), 4 * s, 3 * s, link);

    // Node kapal (emerald) + node Syahbandar (cyan).
    final ship = Paint()..color = const Color(0xFF10B981);
    canvas.drawCircle(Offset(18 * s, 46 * s), 6 * s, ship);
    canvas.drawCircle(Offset(32 * s, 20 * s), 6 * s, ship);
    canvas.drawCircle(
        Offset(48 * s, 42 * s), 6 * s, Paint()..color = AppColors.cyan);

    // Tanda plus pada Syahbandar: M48 38 v8  m-3 -5 h6.
    final plus = Paint()
      ..color = AppColors.surface
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.8 * s;
    canvas.drawLine(Offset(48 * s, 38 * s), Offset(48 * s, 46 * s), plus);
    canvas.drawLine(Offset(45 * s, 41 * s), Offset(51 * s, 41 * s), plus);
  }

  void _dashedLine(Canvas canvas, Offset a, Offset b, double dash, double gap,
      Paint paint) {
    final total = (b - a).distance;
    final dir = (b - a) / total;
    var pos = 0.0;
    while (pos < total) {
      final end = math.min(pos + dash, total);
      canvas.drawLine(a + dir * pos, a + dir * end, paint);
      pos = end + gap;
    }
  }

  @override
  bool shouldRepaint(_LogoPainter oldDelegate) =>
      oldDelegate.withBackground != withBackground;
}
