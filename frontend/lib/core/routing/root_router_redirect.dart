import 'package:flutter/widgets.dart';
import 'package:frontend/core/routing/auth_guard.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:go_router/go_router.dart';

/// Композиция корневого [GoRouter.redirect] (единый слот [GoRouter.redirect]):
/// 1) авторизация для всего префикса **`/projects`** (в go_router v17 **`redirect`**
/// родительского [GoRoute] **не** наследуется дочерними путями — см. задачу **10.7**);
/// 2) нормализация неизвестного сегмента дашборда;
/// 3) далее — другие глобальные правила через явную цепочку (`??` / последовательные `if`).
String? rootRouterRedirect(BuildContext context, GoRouterState state) {
  // Сравнение префикса регистрозависимо (RFC 3986 / go_router); `/Projects/...` не матчится.
  final path = state.uri.path;
  if (path == '/projects' || path.startsWith('/projects/')) {
    final authRedirect = authGuard(context, state);
    if (authRedirect != null) {
      return authRedirect;
    }
  }
  return projectDashboardUnknownShellBranchRedirect(state);
}
