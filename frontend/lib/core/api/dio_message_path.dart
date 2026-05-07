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

final RegExp _taskMessagesPath = RegExp(
  r'/tasks/[^/]+/messages' + kApiPathRestBoundary,
);

final RegExp _taskCorrectPath = RegExp(
  r'/tasks/[^/]+/correct' + kApiPathRestBoundary,
);

final RegExp _projectTasksListPath = RegExp(
  '^'
  r'(?:/api/v\d+)?'
  r'/projects/[^/]+/tasks'
  r'(?:$|\?)',
);

final RegExp _taskResourcePath = RegExp(
  '^'
  r'(?:/api/v\d+)?'
  r'/tasks/[^/]+'
  '$kApiPathRestBoundary',
);

/// Пути `/conversations/{id}/messages` — пользовательский контент; не логировать тела (review.md §1).
bool isConversationMessagesApiPath(String requestPath) {
  return _conversationMessagesPath.hasMatch(normalizedApiPath(requestPath));
}

/// Пути `/tasks/{id}/messages` — сообщения задачи; не логировать тела (PII).
bool isTaskMessagesApiPath(String requestPath) {
  return _taskMessagesPath.hasMatch(normalizedApiPath(requestPath));
}

/// `POST /tasks/{id}/correct` — пользовательский текст коррекции; не логировать тело в debug.
bool isTaskCorrectApiPath(String requestPath) {
  return _taskCorrectPath.hasMatch(normalizedApiPath(requestPath));
}

/// Список/создание чатов проекта: `/projects/{id}/conversations` (не суффиксы вроде `-archive`).
bool isProjectConversationsListPath(String requestPath) {
  return _projectConversationsListPath.hasMatch(normalizedApiPath(requestPath));
}

/// Список/создание задач проекта: `/projects/{id}/tasks` (дисамбигуация 404).
bool isProjectTasksListPath(String requestPath) {
  return _projectTasksListPath.hasMatch(normalizedApiPath(requestPath));
}

/// Любой ресурс задачи по id: `/tasks/{id}`, `/tasks/{id}/pause`, `/tasks/{id}/messages`, …
bool isTaskResourceApiPath(String requestPath) {
  return _taskResourcePath.hasMatch(normalizedApiPath(requestPath));
}
