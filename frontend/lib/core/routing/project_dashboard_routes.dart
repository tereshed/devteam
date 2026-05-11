import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/chat/presentation/screens/chat_conversation_placeholder_screen.dart';
import 'package:frontend/features/chat/presentation/screens/chat_screen.dart';
import 'package:frontend/features/projects/presentation/screens/project_settings_screen.dart';
import 'package:frontend/features/projects/presentation/widgets/project_destination_placeholder.dart';
import 'package:frontend/features/tasks/presentation/screens/task_detail_screen.dart';
import 'package:frontend/features/tasks/presentation/screens/tasks_list_screen.dart';
import 'package:frontend/features/team/presentation/screens/team_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// [Navigator] ветки «Задачи» StatefulShellRoute (`/projects/:id/tasks/...`).
///
/// Cross-branch `push` из чата выполняется через
/// `GoRouter.of(projectDashboardShellTasksNavigatorKey.currentContext!).push(...)` —
/// контекст чата относится к другому вложенному navigator (см. `_openTaskDetailFromChatShell`).
///
/// **Инвариант:** в дереве одновременно может существовать только один [Navigator] с этим ключом.
/// Тесты должны монтировать один дашборд на `pumpWidget`; параллельные деревья / превью с вторым
/// shell приведут к `Duplicate GlobalKey`. При подозрении на flake убедиться, что после теста дерево
/// снято (`pumpWidget`/`tearDown`) и нет висящих таймеров оверлея.
final GlobalKey<NavigatorState> projectDashboardShellTasksNavigatorKey =
    GlobalKey<NavigatorState>(debugLabel: 'projectDashboardShellTasks');

/// Сегмент URL вкладки «Задачи» в shell (`/projects/:id/tasks`).
/// Должен совпадать с соответствующим элементом [projectDashboardShellBranchPaths].
const String projectDashboardShellBranchTasksSegment = 'tasks';

/// Сегмент URL вкладки «Команда» в shell (`/projects/:id/team`).
/// Должен совпадать с соответствующим элементом [projectDashboardShellBranchPaths].
const String projectDashboardShellBranchTeamSegment = 'team';

/// Сегмент URL вкладки «Настройки» в shell (`/projects/:id/settings`).
/// Должен совпадать с соответствующим элементом [projectDashboardShellBranchPaths].
const String projectDashboardShellBranchSettingsSegment = 'settings';

/// Сегмент URL после `/projects/:id` для веток shell (порядок = порядок вкладок).
/// Единственный источник имён путей для [buildProjectDashboardShellBranches] и редиректов.
const List<String> projectDashboardShellBranchPaths = [
  'chat',
  projectDashboardShellBranchTasksSegment,
  projectDashboardShellBranchTeamSegment,
  projectDashboardShellBranchSettingsSegment,
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

/// Невалидный `:taskId` (не UUID) → `/projects/:id/tasks` без потери query.
String? projectDashboardTaskDetailRedirect(
  BuildContext context,
  GoRouterState state,
) {
  final id = state.pathParameters['id'];
  final taskId = state.pathParameters['taskId'];
  if (id == null || taskId == null) {
    return null;
  }
  if (!isValidUuid(taskId)) {
    return projectDashboardRedirectPreservingQuery(
      state,
      '/projects/$id/$projectDashboardShellBranchTasksSegment',
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
          if (projectDashboardShellBranchPaths[i] ==
              projectDashboardShellBranchTasksSegment)
            GoRoute(
              path: projectDashboardShellBranchTasksSegment,
              pageBuilder: (context, state) {
                final projectId = state.pathParameters['id']!;
                return NoTransitionPage(
                  key: state.pageKey,
                  child: TasksListScreen(projectId: projectId),
                );
              },
              routes: [
                GoRoute(
                  path: ':taskId',
                  redirect: projectDashboardTaskDetailRedirect,
                  pageBuilder: (context, state) {
                    final projectId = state.pathParameters['id']!;
                    final taskId = state.pathParameters['taskId']!;
                    return NoTransitionPage(
                      key: state.pageKey,
                      child: TaskDetailScreen(
                        projectId: projectId,
                        taskId: taskId,
                      ),
                    );
                  },
                ),
              ],
            )
          else if (projectDashboardShellBranchPaths[i] ==
              projectDashboardShellBranchTeamSegment)
            GoRoute(
              path: projectDashboardShellBranchTeamSegment,
              pageBuilder: (context, state) {
                final projectId = state.pathParameters['id']!;
                return NoTransitionPage(
                  key: state.pageKey,
                  child: TeamScreen(projectId: projectId),
                );
              },
            )
          else if (projectDashboardShellBranchPaths[i] ==
              projectDashboardShellBranchSettingsSegment)
            GoRoute(
              path: projectDashboardShellBranchSettingsSegment,
              pageBuilder: (context, state) {
                final projectId = state.pathParameters['id']!;
                return NoTransitionPage(
                  key: state.pageKey,
                  child: ProjectSettingsScreen(projectId: projectId),
                );
              },
            )
          else
            GoRoute(
              path: projectDashboardShellBranchPaths[i],
              pageBuilder: (context, state) {
                return NoTransitionPage(
                  key: state.pageKey,
                  child: ProjectDestinationPlaceholder(
                    title: entries[i].title(
                      requireAppLocalizations(
                        context,
                        where: 'project_dashboard_shell',
                      ),
                    ),
                  ),
                );
              },
            ),
        ],
      ),
  ];
}
