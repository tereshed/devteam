import 'package:flutter/material.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:frontend/features/projects/presentation/screens/project_dashboard_screen.dart';
import 'package:go_router/go_router.dart';

/// Тот же контракт, что [AppRouter]: валидация id, редирект, ветки из [buildProjectDashboardShellBranches].
const String kTestProjectUuid = '550e8400-e29b-41d4-a716-446655440000';

final GlobalKey<NavigatorState> kTestShellChatKey =
    GlobalKey<NavigatorState>(debugLabel: 'testShellChat');
final GlobalKey<NavigatorState> kTestShellTasksKey =
    GlobalKey<NavigatorState>(debugLabel: 'testShellTasks');
final GlobalKey<NavigatorState> kTestShellTeamKey =
    GlobalKey<NavigatorState>(debugLabel: 'testShellTeam');
final GlobalKey<NavigatorState> kTestShellSettingsKey =
    GlobalKey<NavigatorState>(debugLabel: 'testShellSettings');

/// [GoRouter] с дашбордом проекта (для тестов; без authGuard).
GoRouter buildProjectDashboardTestRouter({
  required String initialLocation,
  List<RouteBase> routesBeforeProjects = const [],
}) {
  return GoRouter(
    initialLocation: initialLocation,
    redirect: (_, GoRouterState state) =>
        projectDashboardUnknownShellBranchRedirect(state),
    routes: [
      ...routesBeforeProjects,
      GoRoute(
        path: '/projects',
        builder: (context, state) => const Scaffold(
          body: Text('__TEST_PROJECTS_LIST__'),
        ),
        routes: [
          GoRoute(
            path: ':id',
            redirect: projectDashboardDetailRedirect,
            routes: [
              StatefulShellRoute(
                builder: (context, state, navigationShell) {
                  final id = state.pathParameters['id']!;
                  return ProjectDashboardScreen(
                    projectId: id,
                    navigationShell: navigationShell,
                  );
                },
                navigatorContainerBuilder: (
                  BuildContext context,
                  StatefulNavigationShell navigationShell,
                  List<Widget> children,
                ) {
                  // Синхронно с [AppRouter]: см. комментарий в app_router.dart
                  // (navigatorContainerBuilder у StatefulShellRoute).
                  return children[navigationShell.currentIndex];
                },
                branches: buildProjectDashboardShellBranches(
                  chatNavigatorKey: kTestShellChatKey,
                  tasksNavigatorKey: kTestShellTasksKey,
                  teamNavigatorKey: kTestShellTeamKey,
                  settingsNavigatorKey: kTestShellSettingsKey,
                ),
              ),
            ],
          ),
        ],
      ),
    ],
  );
}
