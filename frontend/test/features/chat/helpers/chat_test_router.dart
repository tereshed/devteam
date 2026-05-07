import 'package:go_router/go_router.dart';

import '../../projects/helpers/project_dashboard_test_router.dart';

/// Deep-link и placeholder как в [AppRouter] / **10.9** [`project_dashboard_test_router`] —
/// без authGuard.
GoRouter buildChatTestRouter({
  required String initialLocation,
  List<RouteBase> routesBeforeProjects = const [],
}) =>
    buildProjectDashboardTestRouter(
      initialLocation: initialLocation,
      routesBeforeProjects: routesBeforeProjects,
    );

/// `/projects/:projectId/chat/:conversationId`
String chatTestPathConversation(String projectId, String conversationId) =>
    '/projects/$projectId/chat/$conversationId';

/// `/projects/:projectId/chat` — без `:conversationId`, см. [ChatConversationPlaceholderScreen].
String chatTestPathChatPlaceholder(String projectId) =>
    '/projects/$projectId/chat';
