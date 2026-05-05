import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/chat/presentation/screens/chat_conversation_placeholder_screen.dart';
import 'package:frontend/features/chat/presentation/screens/chat_screen.dart';
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

/// Дефолтная ветка после `/projects/:id` (редирект с корня дашборда и «неизвестного» сегмента).
///
/// **Инвариант:** всегда совпадает с [projectDashboardShellBranchPaths.first] — проверяется в тестах.
const String projectDashboardDefaultBranch = 'chat';

/// Имена маршрутов GoRouter для сегмента `/projects` (без дублирования литералов).
abstract final class ProjectRouteNames {
  static const projects = 'projects';
  static const projectsNew = 'projects_new';
  static const projectsDetail = 'projects_detail';
}

/// Редирект на [newPath] с сохранением query и fragment из [state] ([Uri.replace] оставляет непереданные компоненты).
String projectDashboardRedirectPreservingQuery(
  GoRouterState state,
  String newPath,
) {
  return state.uri.replace(path: newPath).toString();
}

/// Редирект с корня [GoRouter]: при пути `/projects/:id/<x>`, где `<x>` не входит в
/// [projectDashboardShellBranchPaths], дочерний [GoRoute] под `:id` не матчится целиком —
/// route-level [projectDashboardDetailRedirect] для такого URL не выполняется. Без перехвата на
/// уровне [GoRouter.redirect] пользователь попадает в [GoRouter.errorBuilder]. См. задачу
/// **10.7** (`docs/tasks/10.7-gorouter-projects-routes.md`, «Обоснование глобального redirect»).
///
/// **Краевой случай (Sprint 10):** путь `/projects/:id/<branch>/<extra>` при неизвестном
/// `<branch>` — [projectDashboardUnknownShellBranchRedirect]. Ветка `chat` с
/// [projectDashboardChatConversationRedirect] на `:conversationId` отбрасывает не-UUID сегмент
/// на `/projects/:id/chat`.
String? projectDashboardUnknownShellBranchRedirect(GoRouterState state) {
  final segs = state.uri.pathSegments;
  if (segs.length < 3 || segs[0] != 'projects') {
    return null;
  }
  final id = segs[1];
  if (!isValidUuid(id)) {
    return null;
  }
  if (!projectDashboardShellBranchPaths.contains(segs[2])) {
    return projectDashboardRedirectPreservingQuery(
      state,
      '/projects/$id/$projectDashboardDefaultBranch',
    );
  }
  return null;
}

/// Редирект под `/projects/:id`: невалидный id → список; голый id → [projectDashboardDefaultBranch].
String? projectDashboardDetailRedirect(
  BuildContext context,
  GoRouterState state,
) {
  final id = state.pathParameters['id'];
  if (id == null) {
    return null;
  }
  if (!isValidUuid(id)) {
    return '/projects';
  }
  // Корень дашборда: `/projects/:id` и `/projects/:id/` — через нормализацию пути
  // (pathSegments для trailing slash даёт лишний пустой сегмент в некоторых Uri).
  final normalizedPath = _stripSingleTrailingSlash(state.uri.path);
  if (normalizedPath == '/projects/$id') {
    return projectDashboardRedirectPreservingQuery(
      state,
      '/projects/$id/$projectDashboardDefaultBranch',
    );
  }
  return null;
}

/// Нормализация для сравнения корня дашборда: `/projects/:id` и `/projects/:id/` — один путь.
String _stripSingleTrailingSlash(String path) {
  if (path.length > 1 && path.endsWith('/')) {
    return path.substring(0, path.length - 1);
  }
  return path;
}

/// Невалидный `:conversationId` (не UUID) → `/projects/:id/chat` без потери query.
String? projectDashboardChatConversationRedirect(
  BuildContext context,
  GoRouterState state,
) {
  final id = state.pathParameters['id'];
  final conv = state.pathParameters['conversationId'];
  if (id == null || conv == null) {
    return null;
  }
  if (!isValidUuid(conv)) {
    return projectDashboardRedirectPreservingQuery(
      state,
      '/projects/$id/chat',
    );
  }
  return null;
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

  if (entries.length != projectDashboardShellBranchPaths.length) {
    throw StateError(
      'buildProjectDashboardShellBranches: entries.length (${entries.length}) must '
      'equal projectDashboardShellBranchPaths.length '
      '(${projectDashboardShellBranchPaths.length})',
    );
  }

  return [
    StatefulShellBranch(
      navigatorKey: entries[0].key,
      routes: [
        GoRoute(
          path: projectDashboardShellBranchPaths[0],
          pageBuilder: (context, state) {
            final id = state.pathParameters['id']!;
            return NoTransitionPage(
              key: state.pageKey,
              child: ChatConversationPlaceholderScreen(projectId: id),
            );
          },
          routes: [
            GoRoute(
              path: ':conversationId',
              redirect: projectDashboardChatConversationRedirect,
              pageBuilder: (context, state) {
                final id = state.pathParameters['id']!;
                final convId = state.pathParameters['conversationId']!;
                return NoTransitionPage(
                  key: state.pageKey,
                  child: ChatScreen(
                    projectId: id,
                    conversationId: convId,
                  ),
                );
              },
            ),
          ],
        ),
      ],
    ),
    for (var i = 1; i < entries.length; i++)
      StatefulShellBranch(
        navigatorKey: entries[i].key,
        routes: [
          GoRoute(
            path: projectDashboardShellBranchPaths[i],
            pageBuilder: (context, state) => NoTransitionPage(
              key: state.pageKey,
              child: ProjectDestinationPlaceholder(
                title: entries[i].title(
                  requireAppLocalizations(context, where: 'project_dashboard_shell'),
                ),
              ),
            ),
          ),
        ],
      ),
  ];
}
