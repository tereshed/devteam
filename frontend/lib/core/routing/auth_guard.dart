import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';

/// AuthGuard проверяет авторизацию пользователя перед доступом к защищенным маршрутам
///
/// Используется в go_router для автоматического перенаправления
/// неавторизованных пользователей на страницу входа.
String? authGuard(BuildContext context, GoRouterState state) {
  final container = ProviderScope.containerOf(context);
  final authState = container.read(authControllerProvider);

  // Проверяем состояние авторизации
  return authState.when(
    data: (user) {
      // Если пользователь авторизован, разрешаем доступ
      if (user != null) {
        return null;
      }
      // Если не авторизован, перенаправляем на лендинг
      return '/';
    },
    loading: () {
      // Во время загрузки разрешаем доступ (можно показать loading)
      return null;
    },
    error: (error, stackTrace) {
      // При ошибке перенаправляем на лендинг
      return '/';
    },
  );
}
