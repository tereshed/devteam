import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/root_router_redirect.dart';
import 'package:frontend/features/auth/domain/models/user_model.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:go_router/go_router.dart';

// `(_, _)` в builder — отдельные wildcard-параметры (Dart ≥3.7). См. environment.sdk
// в pubspec.yaml (`^3.10.0`).

/// Подклассы [AuthController] только с [build] — без токенов. Если в продовом
/// [AuthController.build] появятся обязательные init/side effects, эти стабы нужно
/// пересобрать (например общий `super`-вызов или hand-written mock), иначе тесты
/// молча разойдутся с продом.
class _GuestAuth extends AuthController {
  @override
  Future<UserModel?> build() async => null;
}

class _LoggedInAuth extends AuthController {
  @override
  Future<UserModel?> build() async => const UserModel(
    id: 'u1',
    email: 'a@b.c',
    role: 'user',
  );
}

void main() {
  testWidgets(
    'guest: rootRouterRedirect для /projects/new уводит на / (auth до дочерних GoRoute)',
    (tester) async {
      late final GoRouter router;
      router = GoRouter(
        redirect: rootRouterRedirect,
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => const Scaffold(body: Text('__ROOT__')),
          ),
          GoRoute(
            path: '/projects',
            builder: (_, _) => const Scaffold(body: Text('__LIST__')),
            routes: [
              GoRoute(
                path: 'new',
                builder: (_, _) => const Scaffold(body: Text('__NEW__')),
              ),
            ],
          ),
        ],
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_GuestAuth.new),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(MaterialApp)),
      );
      // Пока auth в loading, [authGuard] не редиректит — дождаться AsyncData.
      await container.read(authControllerProvider.future);
      router.go('/projects/new');
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/');
      expect(find.text('__ROOT__'), findsOneWidget);
    },
  );

  testWidgets(
    'авторизованный пользователь: /projects/new не режется auth в rootRouterRedirect',
    (tester) async {
      late final GoRouter router;
      router = GoRouter(
        redirect: rootRouterRedirect,
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => const Scaffold(body: Text('__ROOT__')),
          ),
          GoRoute(
            path: '/projects',
            builder: (_, _) => const Scaffold(body: Text('__LIST__')),
            routes: [
              GoRoute(
                path: 'new',
                builder: (_, _) => const Scaffold(body: Text('__NEW__')),
              ),
            ],
          ),
        ],
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_LoggedInAuth.new),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(MaterialApp)),
      );
      // Пока auth в loading, [authGuard] не редиректит — дождаться AsyncData.
      await container.read(authControllerProvider.future);
      router.go('/projects/new');
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects/new');
      expect(find.text('__NEW__'), findsOneWidget);
    },
  );
}
