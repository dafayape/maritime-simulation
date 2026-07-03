import 'package:flutter/material.dart';

import 'ui/core/app_theme.dart';
import 'ui/features/splash/splash_screen.dart';

class MaritimNodeApp extends StatelessWidget {
  const MaritimNodeApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Maritim Node',
      debugShowCheckedModeBanner: false,
      theme: buildAppTheme(),
      home: const SplashScreen(),
    );
  }
}
