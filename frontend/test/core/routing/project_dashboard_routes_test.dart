import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../features/projects/helpers/project_dashboard_test_router.dart';
import '../../features/projects/helpers/project_fixtures.dart';

void main() {
  test('projectDashboardDefaultBranch совпадает с первой веткой (SSOT)', () {
    expect(
      projectDashboardDefaultBranch,
      projectDashboardShellBranchPaths.first,
    );
  });
  test(
    'projectDashboardShellBranchPaths — длина совпадает с buildProjectDashboardShellBranches',
    () {
      final branches = buildProjectDashboardShellBranches(
        chatNavigatorKey: kTestShellChatKey,
        tasksNavigatorKey: kTestShellTasksKey,
        teamNavigatorKey: kTestShellTeamKey,
        settingsNavigatorKey: kTestShellSettingsKey,
      );
      expect(branches.length, projectDashboardShellBranchPaths.length);
    },
  );

  testWidgets(
    '/projects/new матчится литералом new, не как :id (smoke)',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/new',
      );
      await tester.pumpWidget(
        MaterialApp.router(routerConfig: router),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects/new');
      expect(find.text('__TEST_PROJECTS_NEW__'), findsOneWidget);
    },
  );

  testWidgets(
    'редирект /projects/:id?from=x сохраняет query на целевом URL',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid?from=x',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'Q'),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        router.state.uri.path,
        '/projects/$kTestProjectUuid/$projectDashboardDefaultBranch',
      );
      expect(router.state.uri.queryParameters['from'], 'x');
    },
  );

  testWidgets(
    'редирект с корня дашборда сохраняет fragment (#)',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid?from=x#section',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'Frag'),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.fragment, 'section');
      expect(router.state.uri.queryParameters['from'], 'x');
    },
  );

  testWidgets(
    'Sprint 10: /projects/:id/chat/extra без вложенного маршрута → errorBuilder + l10n',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat/extra',
      );
      await tester.pumpWidget(
        MaterialApp.router(
          routerConfig: router,
          localizationsDelegates: const [
            AppLocalizations.delegate,
            GlobalMaterialLocalizations.delegate,
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          supportedLocales: const [Locale('en')],
        ),
      );
      await tester.pumpAndSettle();
      // Контекст [MaterialApp] выше [Localizations] — [AppLocalizations.of] даст null.
      final ctx = tester.element(find.byType(Scaffold));
      expect(
        find.text(AppLocalizations.of(ctx)!.routerNavigationError),
        findsOneWidget,
      );
    },
  );
}
