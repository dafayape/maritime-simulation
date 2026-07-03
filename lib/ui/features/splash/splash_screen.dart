import 'dart:async';

import 'package:flutter/material.dart';

import '../../core/app_logo.dart';
import '../../core/app_theme.dart';
import '../setup/setup_screen.dart';

/// Splash in-app: menyambung mulus dari splash native (warna latar sama,
/// #020617), memainkan animasi skala+fade logo vektor, lalu berpindah ke
/// SetupScreen. Menghormati preferensi reduce-motion pengguna.
class SplashScreen extends StatefulWidget {
  const SplashScreen({super.key});

  @override
  State<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends State<SplashScreen>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 800),
  );
  late final Animation<double> _scale = Tween<double>(begin: 0.86, end: 1)
      .animate(CurvedAnimation(parent: _controller, curve: Curves.easeOutCubic));
  late final Animation<double> _fade =
      CurvedAnimation(parent: _controller, curve: Curves.easeOut);
  Timer? _navTimer;

  @override
  void initState() {
    super.initState();
    _controller.forward();
    _navTimer = Timer(const Duration(milliseconds: 1500), _goToSetup);
  }

  void _goToSetup() {
    if (!mounted) return;
    Navigator.of(context).pushReplacement(
      PageRouteBuilder(
        pageBuilder: (_, _, _) => const SetupScreen(),
        transitionsBuilder: (_, animation, _, child) =>
            FadeTransition(opacity: animation, child: child),
        transitionDuration: const Duration(milliseconds: 350),
      ),
    );
  }

  @override
  void dispose() {
    _navTimer?.cancel();
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final reduceMotion = MediaQuery.of(context).disableAnimations;
    final logo = Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        const AppLogo(size: 132),
        const SizedBox(height: 26),
        const Text(
          'Maritim Node',
          style: TextStyle(
            fontSize: 26,
            fontWeight: FontWeight.w800,
            color: AppColors.text,
            letterSpacing: 0.3,
          ),
        ),
        const SizedBox(height: 8),
        Text(
          'Maritime LoRa Mesh Network Simulator',
          style: TextStyle(
            fontSize: 13,
            color: AppColors.textMuted.withValues(alpha: 0.9),
            letterSpacing: 0.4,
          ),
        ),
      ],
    );

    return Scaffold(
      backgroundColor: AppColors.bg,
      body: Center(
        child: reduceMotion
            ? logo
            : FadeTransition(
                opacity: _fade,
                child: ScaleTransition(scale: _scale, child: logo),
              ),
      ),
    );
  }
}
