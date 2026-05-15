// widget_test_harness.dart — общий harness для widget-тестов.
//
// Заворачивает [child] в [ProviderScope] + [MaterialApp] c подключенными
// AppLocalizations. Единая точка изменения, чтобы при добавлении новых
// localization-delegates или global theme'ов мы не правили N тестов.
//
// Если экрану нужен GoRouter (например, для context.go в onTap'ах),
// используй [pumpAppWidgetWithRouter] и передай свой `routerConfig`.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Заворачивает [child] в стандартный harness и накачивает до settled state.
///
/// [overrides] — Riverpod-overrides (repos, FutureProviders с тестовыми
/// данными). [screenSize] — позволяет растянуть viewport для экранов с
/// длинной формой, иначе кнопки уходят за пределы default 800x600 и
/// `tester.tap` промахивается. Если задан, сбрасывается в tearDown.
Future<void> pumpAppWidget(
  WidgetTester tester, {
  required Widget child,
  List<Override> overrides = const [],
  Size? screenSize,
}) async {
  if (screenSize != null) {
    tester.view.physicalSize = screenSize;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }
  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: overrides,
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: child,
      ),
    ),
  );
  await tester.pumpAndSettle();
}

/// Версия для экранов, использующих [GoRouter] (например, context.go).
Future<void> pumpAppWidgetWithRouter(
  WidgetTester tester, {
  required GoRouter routerConfig,
  List<Override> overrides = const [],
  Size? screenSize,
}) async {
  if (screenSize != null) {
    tester.view.physicalSize = screenSize;
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }
  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: overrides,
      child: MaterialApp.router(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: routerConfig,
      ),
    ),
  );
  await tester.pumpAndSettle();
}
