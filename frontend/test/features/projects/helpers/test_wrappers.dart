import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
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

// Для виджетов без GoRouter (ProjectStatusChip)
Widget wrapSimple(Widget child) => MaterialApp(
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ],
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en'),
      home: Scaffold(body: child),
    );

// Для виджетов с GoRouter (ProjectCard, ProjectsListScreen)
Widget wrapRouter({
  required Widget Function(BuildContext, GoRouterState) builder,
  List<Override> overrides = const [],
}) =>
    ProviderScope(
      // Без retry ошибки AsyncNotifier не «залипают» в бесконечных повторах в тестах.
      retry: (_, _) => null,
      overrides: overrides,
      child: MaterialApp.router(
        localizationsDelegates: const [
          AppLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
        ],
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('en'),
        routerConfig: GoRouter(
          routes: [
            GoRoute(path: '/', builder: builder),
            GoRoute(
              path: '/projects/:id',
              builder: (context, state) => const SizedBox(),
            ),
            GoRoute(
              path: '/projects/new',
              builder: (context, state) => const SizedBox(),
            ),
          ],
        ),
      ),
    );
