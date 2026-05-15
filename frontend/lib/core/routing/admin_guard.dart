import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:go_router/go_router.dart';

/// AdminGuard ограничивает доступ к /admin/* роутам только пользователям с ролью `admin`.
///
/// Если auth-state не загружен (loading) — пропускаем, чтобы не блокировать инициализацию.
/// Если пользователь не авторизован — отправляем на лендинг (как [authGuard]).
/// Если авторизован, но роль != `admin` — на `/dashboard`.
String? adminGuard(BuildContext context, GoRouterState state) {
  final container = ProviderScope.containerOf(context);
  final authState = container.read(authControllerProvider);

  return authState.when(
    data: (user) {
      if (user == null) {
        return '/';
      }
      if (user.role == 'admin') {
        return null;
      }
      return '/dashboard';
    },
    loading: () => null,
    error: (_, _) => '/',
  );
}
