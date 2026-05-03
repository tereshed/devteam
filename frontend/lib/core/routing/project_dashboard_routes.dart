import 'package:flutter/material.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/projects/presentation/widgets/project_destination_placeholder.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Сегмент URL после `/projects/:id` для веток shell (порядок = порядок вкладок).
/// Единственный источник имён путей для [buildProjectDashboardShellBranches] и редиректов.
const List<String> projectDashboardShellBranchPaths = [
  'chat',
  'tasks',
  'team',
  'settings',
];

/// Редирект с корня [GoRouter]: при `/projects/:id/<неизвестно>` дочерний матч не строится,
/// [projectDashboardDetailRedirect] на `:id` не вызывается — без этого пользователь попадает в [GoRouter.errorBuilder].
String? projectDashboardUnknownShellBranchRedirect(GoRouterState state) {
  final segs = state.uri.pathSegments;
  if (segs.length < 3 || segs[0] != 'projects') {
    return null;
  }
  final id = segs[1];
  if (!isValidProjectUuid(id)) {
    return null;
  }
  if (!projectDashboardShellBranchPaths.contains(segs[2])) {
    return '/projects/$id/chat';
  }
  return null;
}

/// Редирект под `/projects/:id`: невалидный id → список; голый id → ветка chat.
String? projectDashboardDetailRedirect(
  BuildContext context,
  GoRouterState state,
) {
  final id = state.pathParameters['id'];
  if (id == null) {
    return null;
  }
  if (!isValidProjectUuid(id)) {
    return '/projects';
  }
  final path = _stripSingleTrailingSlash(state.uri.path);
  if (path == '/projects/$id') {
    return '/projects/$id/chat';
  }
  return null;
}

/// Нормализация для сравнения корня дашборда: `/projects/:id` и `/projects/:id/` считаются одним путём.
String _stripSingleTrailingSlash(String path) {
  if (path.length > 1 && path.endsWith('/')) {
    return path.substring(0, path.length - 1);
  }
  return path;
}

/// Ветки [StatefulShellRoute] дашборда проекта (single source для prod и тестов).
List<StatefulShellBranch> buildProjectDashboardShellBranches({
  required GlobalKey<NavigatorState> chatNavigatorKey,
  required GlobalKey<NavigatorState> tasksNavigatorKey,
  required GlobalKey<NavigatorState> teamNavigatorKey,
  required GlobalKey<NavigatorState> settingsNavigatorKey,
}) {
  final entries =
      <
        ({
          GlobalKey<NavigatorState> key,
          String Function(AppLocalizations) title,
        })
      >[
        (
          key: chatNavigatorKey,
          title: (l) => l.projectDashboardChat,
        ),
        (
          key: tasksNavigatorKey,
          title: (l) => l.projectDashboardTasks,
        ),
        (
          key: teamNavigatorKey,
          title: (l) => l.projectDashboardTeam,
        ),
        (
          key: settingsNavigatorKey,
          title: (l) => l.projectDashboardSettings,
        ),
      ];

  assert(
    entries.length == projectDashboardShellBranchPaths.length,
    'ветки shell и projectDashboardShellBranchPaths должны совпадать по длине',
  );

  return [
    for (var i = 0; i < entries.length; i++)
      StatefulShellBranch(
        navigatorKey: entries[i].key,
        routes: [
          GoRoute(
            path: projectDashboardShellBranchPaths[i],
            pageBuilder: (context, state) => NoTransitionPage(
              key: state.pageKey,
              child: ProjectDestinationPlaceholder(
                title: entries[i].title(AppLocalizations.of(context)!),
              ),
            ),
          ),
        ],
      ),
  ];
}
