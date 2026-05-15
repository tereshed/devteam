import 'package:flutter/material.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/routing/auth_guard.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:frontend/core/routing/root_router_redirect.dart';
import 'package:frontend/core/routing/router_error_screen.dart';
import 'package:frontend/features/admin/agents_v2/presentation/screens/agent_v2_detail_screen.dart';
import 'package:frontend/features/admin/agents_v2/presentation/screens/agents_v2_list_screen.dart';
import 'package:frontend/features/admin/prompts/presentation/screens/prompt_edit_screen.dart';
import 'package:frontend/features/admin/prompts/presentation/screens/prompts_list_screen.dart';
import 'package:frontend/features/admin/worktrees_v2/presentation/screens/worktrees_list_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/execution_detail_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/executions_list_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/workflows_list_screen.dart';
import 'package:frontend/features/auth/presentation/screens/api_keys_screen.dart';
import 'package:frontend/features/auth/presentation/screens/dashboard_screen.dart';
import 'package:frontend/features/auth/presentation/screens/login_screen.dart';
import 'package:frontend/features/auth/presentation/screens/profile_screen.dart';
import 'package:frontend/features/auth/presentation/screens/register_screen.dart';
import 'package:frontend/features/landing/presentation/screens/landing_screen.dart';
import 'package:frontend/features/projects/presentation/screens/create_project_screen.dart';
import 'package:frontend/features/projects/presentation/screens/project_dashboard_screen.dart';
import 'package:frontend/features/projects/presentation/screens/projects_list_screen.dart';
import 'package:frontend/features/settings/presentation/screens/global_settings_screen.dart';
import 'package:go_router/go_router.dart';

/// Ключи вложенных [Navigator] для веток дашборда проекта (StatefulShellRoute).
final GlobalKey<NavigatorState> _projectShellChatNavKey =
    GlobalKey<NavigatorState>(debugLabel: 'projectShellChat');
final GlobalKey<NavigatorState> _projectShellTeamNavKey =
    GlobalKey<NavigatorState>(debugLabel: 'projectShellTeam');
final GlobalKey<NavigatorState> _projectShellSettingsNavKey =
    GlobalKey<NavigatorState>(debugLabel: 'projectShellSettings');

/// AppRouter настраивает маршрутизацию приложения
///
/// Использует go_router для навигации на основе URL.
/// Поддерживает глубокие ссылки и SEO для Web версии.
///
/// Разделяет роуты на:
/// - Public routes: доступны всем (/, /login, /register)
/// - Protected routes: требуют авторизации (/dashboard, /profile)
class AppRouter {
  static final GoRouter router = GoRouter(
    initialLocation: '/',
    redirect: rootRouterRedirect,
    routes: [
      // Public routes
      GoRoute(
        path: '/',
        name: 'home',
        // redirect: (context, state) {
        //   final container = ProviderScope.containerOf(context);
        //   final authState = container.read(authControllerProvider);
        //
        //   // Если пользователь уже авторизован, отправляем в dashboard
        //   if (authState.value != null) {
        //     return '/dashboard';
        //   }
        //   return null;
        // },
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const LandingScreen()),
      ),
      GoRoute(
        path: '/login',
        name: 'login',
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const LoginScreen()),
      ),
      GoRoute(
        path: '/register',
        name: 'register',
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const RegisterScreen()),
      ),

      // Protected routes (требуют авторизации)
      GoRoute(
        path: '/dashboard',
        name: 'dashboard',
        redirect: authGuard,
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const DashboardScreen()),
      ),
      GoRoute(
        path: '/profile',
        name: 'profile',
        redirect: authGuard,
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const ProfileScreen()),
      ),
      GoRoute(
        path: AppRoutePaths.profileApiKeys,
        name: 'api_keys',
        redirect: authGuard,
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const ApiKeysScreen()),
      ),
      GoRoute(
        path: AppRoutePaths.settings,
        name: AppRouteNames.globalSettings,
        redirect: authGuard,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const GlobalSettingsScreen(),
        ),
      ),

      GoRoute(
        path: '/projects',
        name: ProjectRouteNames.projects,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const ProjectsListScreen(),
        ),
        routes: [
          GoRoute(
            path: 'new',
            name: ProjectRouteNames.projectsNew,
            pageBuilder: (context, state) => MaterialPage(
              key: state.pageKey,
              child: const CreateProjectScreen(),
            ),
          ),
          GoRoute(
            path: ':id',
            name: ProjectRouteNames.projectsDetail,
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
                  // Только активная ветка: компромисс по state вкладок — см.
                  // docs/tasks/10.7-gorouter-projects-routes.md, «StatefulShellRoute и сохранение state».
                  return children[navigationShell.currentIndex];
                },
                branches: buildProjectDashboardShellBranches(
                  chatNavigatorKey: _projectShellChatNavKey,
                  tasksNavigatorKey: projectDashboardShellTasksNavigatorKey,
                  teamNavigatorKey: _projectShellTeamNavKey,
                  settingsNavigatorKey: _projectShellSettingsNavKey,
                ),
              ),
            ],
          ),
        ],
      ),

      // Admin Routes (в реальном проекте нужен отдельный adminGuard)
      // Agents v2 — реестр LLM/sandbox-агентов (Orchestration v2 / Sprint 17).
      GoRoute(
        path: '/admin/agents-v2',
        name: 'admin_agents_v2',
        redirect: authGuard,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const AgentsV2ListScreen(),
        ),
        routes: [
          GoRoute(
            path: ':id',
            name: 'admin_agents_v2_detail',
            pageBuilder: (context, state) {
              final id = state.pathParameters['id']!;
              return MaterialPage(
                key: state.pageKey,
                child: AgentV2DetailScreen(agentId: id),
              );
            },
          ),
        ],
      ),
      // Worktrees debug — текущие активные git worktree'ы (Orchestration v2 / Sprint 17).
      GoRoute(
        path: '/admin/worktrees',
        name: 'admin_worktrees',
        redirect: authGuard,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const WorktreesListScreen(),
        ),
      ),
      GoRoute(
        path: '/admin/prompts',
        name: 'admin_prompts',
        redirect: authGuard,
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const PromptsListScreen()),
        routes: [
          GoRoute(
            path: ':id',
            name: 'admin_prompts_detail',
            pageBuilder: (context, state) {
              final id = state.pathParameters['id']!;
              return MaterialPage(
                key: state.pageKey,
                child: PromptDetailScreen(promptId: id),
              );
            },
          ),
        ],
      ),
      GoRoute(
        path: '/admin/workflows',
        name: 'admin_workflows',
        redirect: authGuard,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const WorkflowsListScreen(),
        ),
      ),
      GoRoute(
        path: '/admin/executions',
        name: 'admin_executions',
        redirect: authGuard,
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: const ExecutionsListScreen(),
        ),
        routes: [
          GoRoute(
            path: ':id',
            name: 'admin_execution_detail',
            pageBuilder: (context, state) {
              final id = state.pathParameters['id']!;
              return MaterialPage(
                key: state.pageKey,
                child: ExecutionDetailScreen(id: id),
              );
            },
          ),
        ],
      ),
    ],
    // Текст ошибки — [buildRouterErrorScreen] / ключ routerNavigationError в app_*.arb.
    errorBuilder: buildRouterErrorScreen,
  );
}
