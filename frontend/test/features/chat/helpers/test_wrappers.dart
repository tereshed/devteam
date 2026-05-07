import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Фиксированный logical size + tearDown для [tester.view.reset].
void useViewSize(WidgetTester tester, Size size) {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

/// Дерево с чатом: **`AppLocalizations`**, **`MediaQuery.copyWith`** (не «голый» [MediaQueryData]).
Widget wrapChatMaterialApp({
  required Widget home,
  Locale locale = const Locale('en'),
  TextScaler textScaler = TextScaler.noScaling,
  List<Override> overrides = const [],
  ThemeData? theme,
  ThemeData? darkTheme,
  ThemeMode themeMode = ThemeMode.system,
}) =>
    ProviderScope(
      retry: (_, _) => null,
      overrides: overrides,
      child: MaterialApp(
        theme: theme,
        darkTheme: darkTheme,
        themeMode: themeMode,
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        builder: (context, child) {
          final mq = MediaQuery.of(context);
          return MediaQuery(
            data: mq.copyWith(textScaler: textScaler),
            child: child!,
          );
        },
        home: home,
      ),
    );

/// [MaterialApp] без Riverpod — golden и минимальные harness (TaskStatusCard и т.п.).
Widget wrapChatMaterialAppLite({
  required Widget home,
  Locale locale = const Locale('en'),
  ThemeData? theme,
  ThemeData? darkTheme,
  ThemeMode themeMode = ThemeMode.system,
  TextScaler textScaler = TextScaler.noScaling,
}) =>
    MaterialApp(
      theme: theme,
      darkTheme: darkTheme,
      themeMode: themeMode,
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      builder: (context, child) {
        final mq = MediaQuery.of(context);
        return MediaQuery(
          data: mq.copyWith(textScaler: textScaler),
          child: child!,
        );
      },
      home: home,
    );

/// TaskStatusCard: ru, M3 light/dark — делегирует в [wrapChatMaterialAppLite].
Widget wrapTaskStatusRu(
  Widget child, {
  ThemeMode themeMode = ThemeMode.light,
  TextScaler textScaler = TextScaler.noScaling,
  TextDirection direction = TextDirection.ltr,
}) =>
    wrapChatMaterialAppLite(
      locale: const Locale('ru'),
      theme: ThemeData.light(useMaterial3: true),
      darkTheme: ThemeData.dark(useMaterial3: true),
      themeMode: themeMode,
      textScaler: textScaler,
      home: Directionality(
        textDirection: direction,
        child: Center(child: child),
      ),
    );

/// Верхний уровень — синтетический [MediaQuery] для поля ввода (**ChatInput** тесты).
Widget wrapChatInputHarness({
  required Widget body,
  Locale locale = const Locale('en'),
  TextScaler textScaler = TextScaler.noScaling,
}) =>
    ProviderScope(
      retry: (_, _) => null,
      child: MaterialApp(
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        builder: (context, child) {
          final mq = MediaQuery.of(context);
          return MediaQuery(
            data: mq.copyWith(textScaler: textScaler),
            child: child!,
          );
        },
        home: Scaffold(body: body),
      ),
    );

/// Обёртка без Riverpod (локали как в проде).
Widget wrapChatSimple({
  required Widget child,
  Locale locale = const Locale('en'),
  TextScaler textScaler = TextScaler.noScaling,
}) =>
    MaterialApp(
      locale: locale,
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      builder: (context, child) {
        final mq = MediaQuery.of(context);
        return MediaQuery(
          data: mq.copyWith(textScaler: textScaler),
          child: child!,
        );
      },
      home: Scaffold(body: child),
    );

/// [MaterialApp.router] + [GoRouter] для deep-link (**11.11**).
Widget wrapChatDashboardRouter({
  required GoRouter router,
  List<Override> overrides = const [],
  Locale locale = const Locale('en'),
}) =>
    ProviderScope(
      retry: (_, _) => null,
      overrides: overrides,
      child: MaterialApp.router(
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    );
