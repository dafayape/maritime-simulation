import 'package:flutter/material.dart';

/// Palet identik dengan dashboard frontend (Tailwind slate + sky + emerald,
/// lihat frontend/src/app/globals.css) supaya kedua klien terasa satu produk.
abstract final class AppColors {
  static const bg = Color(0xFF020617); // slate-950
  static const surface = Color(0xFF0F172A); // slate-900
  static const surfaceHi = Color(0xFF1E293B); // slate-800
  static const border = Color(0xFF334155); // slate-700
  static const text = Color(0xFFE2E8F0); // slate-200
  static const textMuted = Color(0xFF94A3B8); // slate-400
  static const sky = Color(0xFF38BDF8); // sky-400
  static const emerald = Color(0xFF34D399); // emerald-400
  static const cyan = Color(0xFF22D3EE); // cyan-400
  static const rose = Color(0xFFFB7185); // rose-400
  static const amber = Color(0xFFFBBF24); // amber-400
  static const navy = Color(0xFF1E3A5F); // ombak logo
}

ThemeData buildAppTheme() {
  const scheme = ColorScheme.dark(
    primary: AppColors.sky,
    onPrimary: AppColors.bg,
    secondary: AppColors.emerald,
    onSecondary: AppColors.bg,
    tertiary: AppColors.cyan,
    error: AppColors.rose,
    onError: AppColors.bg,
    surface: AppColors.surface,
    onSurface: AppColors.text,
    surfaceContainerHighest: AppColors.surfaceHi,
    outline: AppColors.border,
    outlineVariant: AppColors.surfaceHi,
  );

  final base = ThemeData(useMaterial3: true, colorScheme: scheme);
  return base.copyWith(
    scaffoldBackgroundColor: AppColors.bg,
    appBarTheme: const AppBarTheme(
      backgroundColor: AppColors.bg,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      centerTitle: false,
      titleTextStyle: TextStyle(
        color: AppColors.text,
        fontSize: 17,
        fontWeight: FontWeight.w600,
      ),
    ),
    cardTheme: CardThemeData(
      color: AppColors.surface,
      surfaceTintColor: Colors.transparent,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: const BorderSide(color: AppColors.surfaceHi),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: AppColors.surfaceHi.withValues(alpha: 0.55),
      labelStyle: const TextStyle(color: AppColors.textMuted),
      helperStyle: const TextStyle(color: AppColors.textMuted, fontSize: 11.5),
      hintStyle: TextStyle(color: AppColors.textMuted.withValues(alpha: 0.7)),
      contentPadding:
          const EdgeInsets.symmetric(horizontal: 14, vertical: 13),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: const BorderSide(color: AppColors.border),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: const BorderSide(color: AppColors.border),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: const BorderSide(color: AppColors.sky, width: 1.6),
      ),
      errorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: const BorderSide(color: AppColors.rose),
      ),
      focusedErrorBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(12),
        borderSide: const BorderSide(color: AppColors.rose, width: 1.6),
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: AppColors.sky,
        foregroundColor: AppColors.bg,
        disabledBackgroundColor: AppColors.surfaceHi,
        disabledForegroundColor: AppColors.textMuted,
        minimumSize: const Size.fromHeight(48),
        textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
        shape:
            RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        foregroundColor: AppColors.sky,
        side: const BorderSide(color: AppColors.border),
        minimumSize: const Size(48, 48),
        shape:
            RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      ),
    ),
    textButtonTheme: TextButtonThemeData(
      style: TextButton.styleFrom(foregroundColor: AppColors.sky),
    ),
    switchTheme: SwitchThemeData(
      thumbColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.bg
            : AppColors.textMuted,
      ),
      trackColor: WidgetStateProperty.resolveWith(
        (states) => states.contains(WidgetState.selected)
            ? AppColors.emerald
            : AppColors.surfaceHi,
      ),
      trackOutlineColor:
          const WidgetStatePropertyAll<Color>(AppColors.border),
    ),
    tabBarTheme: const TabBarThemeData(
      labelColor: AppColors.sky,
      unselectedLabelColor: AppColors.textMuted,
      indicatorColor: AppColors.sky,
      dividerColor: AppColors.surfaceHi,
      labelStyle: TextStyle(fontSize: 13.5, fontWeight: FontWeight.w700),
      unselectedLabelStyle:
          TextStyle(fontSize: 13.5, fontWeight: FontWeight.w500),
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: AppColors.surfaceHi,
      contentTextStyle: const TextStyle(color: AppColors.text),
      behavior: SnackBarBehavior.floating,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: const BorderSide(color: AppColors.border),
      ),
    ),
    dialogTheme: DialogThemeData(
      backgroundColor: AppColors.surface,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: const BorderSide(color: AppColors.surfaceHi),
      ),
    ),
    dividerTheme:
        const DividerThemeData(color: AppColors.surfaceHi, thickness: 1),
    progressIndicatorTheme: const ProgressIndicatorThemeData(
      color: AppColors.sky,
      linearTrackColor: AppColors.surfaceHi,
    ),
    listTileTheme: const ListTileThemeData(
      textColor: AppColors.text,
      iconColor: AppColors.textMuted,
    ),
    textTheme: base.textTheme.apply(
      bodyColor: AppColors.text,
      displayColor: AppColors.text,
    ),
  );
}
