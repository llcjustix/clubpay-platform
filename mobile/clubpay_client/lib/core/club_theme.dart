import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';

/// Dark grouped appearance, measured in logical points like iOS Settings.
abstract final class ClubColors {
  static const background = Color(0xff000000);
  static const surface = Color(0xff1c1c1e);
  static const elevated = Color(0xff2c2c2e);
  static const separator = Color(0xff38383a);
  static const text = Color(0xffffffff);
  static const muted = Color(0xff98989f);
  static const blue = Color(0xff0a84ff);
  static const green = Color(0xff30d158);
  static const orange = Color(0xffff9f0a);
  static const purple = Color(0xffbf5af2);
  static const red = Color(0xffff453a);
}

ThemeData clubTheme() {
  final base = ThemeData(
    brightness: Brightness.dark,
    useMaterial3: true,
    platform: TargetPlatform.iOS,
    fontFamily: '.SF Pro Text',
    fontFamilyFallback: const [
      '-apple-system',
      'BlinkMacSystemFont',
      'Helvetica Neue',
      'Arial',
    ],
    scaffoldBackgroundColor: ClubColors.background,
    colorScheme: const ColorScheme.dark(
      primary: ClubColors.blue,
      onPrimary: Colors.white,
      surface: ClubColors.surface,
      onSurface: ClubColors.text,
      secondary: ClubColors.muted,
      error: ClubColors.red,
      outline: ClubColors.separator,
    ),
  );
  return base.copyWith(
    cupertinoOverrideTheme: const CupertinoThemeData(
      brightness: Brightness.dark,
      primaryColor: ClubColors.blue,
      scaffoldBackgroundColor: Colors.black,
    ),
    splashFactory: NoSplash.splashFactory,
    textTheme: base.textTheme
        .copyWith(
          displaySmall: const TextStyle(
            fontSize: 34,
            height: 1.2,
            fontWeight: FontWeight.w700,
            letterSpacing: .1,
          ),
          headlineLarge: const TextStyle(
            fontSize: 34,
            height: 1.2,
            fontWeight: FontWeight.w700,
            letterSpacing: .1,
          ),
          headlineMedium: const TextStyle(
            fontSize: 28,
            height: 1.2,
            fontWeight: FontWeight.w700,
            letterSpacing: .1,
          ),
          headlineSmall: const TextStyle(
            fontSize: 22,
            height: 1.27,
            fontWeight: FontWeight.w600,
          ),
          titleLarge: const TextStyle(
            fontSize: 22,
            height: 1.27,
            fontWeight: FontWeight.w600,
          ),
          titleMedium: const TextStyle(
            fontSize: 17,
            height: 1.3,
            fontWeight: FontWeight.w600,
            letterSpacing: -.4,
          ),
          bodyLarge: const TextStyle(
            fontSize: 17,
            height: 1.3,
            letterSpacing: -.4,
          ),
          bodyMedium: const TextStyle(
            fontSize: 17,
            height: 1.3,
            letterSpacing: -.4,
          ),
          bodySmall: const TextStyle(
            fontSize: 13,
            height: 1.38,
            letterSpacing: -.1,
          ),
          labelLarge: const TextStyle(
            fontSize: 17,
            fontWeight: FontWeight.w600,
          ),
        )
        .apply(
          fontFamily: '.SF Pro Text',
          fontFamilyFallback: const [
            '-apple-system',
            'BlinkMacSystemFont',
            'Helvetica Neue',
            'Arial',
          ],
          bodyColor: ClubColors.text,
          displayColor: ClubColors.text,
        ),
    appBarTheme: const AppBarTheme(
      backgroundColor: Colors.black,
      surfaceTintColor: Colors.transparent,
      centerTitle: true,
      toolbarHeight: 52,
      titleTextStyle: TextStyle(
        fontFamily: '.SF Pro Text',
        fontFamilyFallback: ['Helvetica Neue', 'Arial'],
        fontSize: 17,
        fontWeight: FontWeight.w600,
        color: Colors.white,
      ),
    ),
    dividerTheme: const DividerThemeData(
      color: ClubColors.separator,
      thickness: .5,
      space: 0,
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: ClubColors.surface,
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(14),
        borderSide: BorderSide.none,
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(14),
        borderSide: BorderSide.none,
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(14),
        borderSide: const BorderSide(color: ClubColors.blue),
      ),
      labelStyle: const TextStyle(color: ClubColors.muted),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        minimumSize: const Size(44, 50),
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 13),
        textStyle: const TextStyle(fontSize: 17, fontWeight: FontWeight.w600),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      ),
    ),
    outlinedButtonTheme: OutlinedButtonThemeData(
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(44, 48),
        backgroundColor: ClubColors.surface,
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        foregroundColor: ClubColors.blue,
        side: BorderSide.none,
        textStyle: const TextStyle(fontSize: 17, fontWeight: FontWeight.w400),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
      ),
    ),
    bottomSheetTheme: const BottomSheetThemeData(
      backgroundColor: ClubColors.surface,
      surfaceTintColor: Colors.transparent,
      showDragHandle: true,
      dragHandleColor: Color(0xff636366),
      dragHandleSize: Size(36, 5),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
    ),
  );
}
