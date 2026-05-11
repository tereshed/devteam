/// Глобальные пути приложения вне `/projects/:id/...` (SSOT для go_router и тестов).
///
/// См. задачу **13.5** (`docs/tasks/13.5-global-settings-screen.md`).
abstract final class AppRoutePaths {
  AppRoutePaths._();

  /// Глобальные настройки LLM-провайдеров (не путать с `/projects/:id/settings`).
  static const String settings = '/settings';

  /// Ключи API продукта DevTeam (MCP), не LLM-провайдеры.
  static const String profileApiKeys = '/profile/api-keys';
}

abstract final class AppRouteNames {
  AppRouteNames._();

  static const String globalSettings = 'global_settings';
}
