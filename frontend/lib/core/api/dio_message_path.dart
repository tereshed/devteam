/// Убирает схему/хост, если [requestPath] — полный URI (как в части [DioException]).
String normalizedApiPath(String requestPath) {
  if (requestPath.contains('://')) {
    return Uri.parse(requestPath).path;
  }
  return requestPath;
}

/// Граница REST-сегмента: конец строки, `/` или `?` (общий суффикс для шаблонов путей).
const String kApiPathRestBoundary = r'(?:/|$|\?)';

final RegExp _conversationMessagesPath = RegExp(
  r'/conversations/[^/]+/messages' + kApiPathRestBoundary,
);

final RegExp _projectConversationsListPath = RegExp(
  r'/projects/[^/]+/conversations' + kApiPathRestBoundary,
);

/// Пути `/conversations/{id}/messages` — пользовательский контент; не логировать тела (review.md §1).
bool isConversationMessagesApiPath(String requestPath) {
  return _conversationMessagesPath.hasMatch(normalizedApiPath(requestPath));
}

/// Список/создание чатов проекта: `/projects/{id}/conversations` (не суффиксы вроде `-archive`).
bool isProjectConversationsListPath(String requestPath) {
  return _projectConversationsListPath.hasMatch(normalizedApiPath(requestPath));
}
