import 'package:flutter/material.dart';
import 'package:frontend/core/routing/auth_guard.dart';
import 'package:frontend/features/admin/prompts/presentation/screens/prompt_edit_screen.dart';
import 'package:frontend/features/admin/prompts/presentation/screens/prompts_list_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/execution_detail_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/executions_list_screen.dart';
import 'package:frontend/features/admin/workflows/presentation/screens/workflows_list_screen.dart';
import 'package:frontend/features/auth/presentation/screens/api_keys_screen.dart';
import 'package:frontend/features/auth/presentation/screens/dashboard_screen.dart';
import 'package:frontend/features/auth/presentation/screens/login_screen.dart';
import 'package:frontend/features/auth/presentation/screens/profile_screen.dart';
import 'package:frontend/features/auth/presentation/screens/register_screen.dart';
import 'package:frontend/features/landing/presentation/screens/landing_screen.dart';
import 'package:go_router/go_router.dart';

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
        path: '/profile/api-keys',
        name: 'api_keys',
        redirect: authGuard,
        pageBuilder: (context, state) =>
            MaterialPage(key: state.pageKey, child: const ApiKeysScreen()),
      ),

      // Admin Routes (в реальном проекте нужен отдельный adminGuard)
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
    // Обработка ошибок роутинга
    errorBuilder: (context, state) {
      // Для ошибок роутинга используем простой текст без локализации,
      // так как это техническая ошибка, не видимая пользователю
      return Scaffold(body: Center(child: Text('Error: ${state.error}')));
    },
  );
}
