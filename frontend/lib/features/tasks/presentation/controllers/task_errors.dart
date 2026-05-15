import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Результат HTTP-мутаций задач для UI (12.3 §84). Ошибки транспорта/4xx чаще пробрасываются дальше.
enum TaskMutationOutcome {
  validationFailed,
  /// Нет поверхности состояния ([AsyncData]) — мутации не выполняем «тихим» HTTP.
  notReady,
  blockedByRealtime,
  /// Cancel race: backend вернул 409 task_already_terminal — задача уже завершена.
  /// UI должен показать info-toast (не красный snack) и обновить state из БД.
  alreadyTerminal,
  completed,
}

/// Заголовок ошибки списка задач (SnackBar / диалог).
String taskListErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    UnauthorizedException _ => l10n.errorUnauthorized,
    ProjectNotFoundException _ => l10n.taskListErrorProjectNotFound,
    TaskForbiddenException _ => l10n.errorForbidden,
    TaskConflictException _ => l10n.taskErrorGeneric,
    TaskUnprocessableException _ => l10n.taskErrorGeneric,
    TaskCancelledException _ => l10n.errorRequestCancelled,
    final TaskApiException e => _taskApiErrorTitle(l10n, e),
    _ => l10n.taskErrorGeneric,
  };
}

/// Заголовок ошибки деталей задачи.
String taskDetailErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    UnauthorizedException _ => l10n.errorUnauthorized,
    TaskDetailProjectMismatchException _ => l10n.taskDetailProjectMismatch,
    TaskNotFoundException _ => l10n.taskDetailErrorTaskNotFound,
    TaskForbiddenException _ => l10n.errorForbidden,
    TaskConflictException _ => l10n.taskErrorGeneric,
    TaskUnprocessableException _ => l10n.taskErrorGeneric,
    TaskCancelledException _ => l10n.errorRequestCancelled,
    final TaskApiException e => _taskApiErrorTitle(l10n, e),
    _ => l10n.taskErrorGeneric,
  };
}

String _taskApiErrorTitle(AppLocalizations l10n, TaskApiException e) {
  if (e.isNetworkTransportError) {
    return l10n.errorNetwork;
  }
  if ((e.statusCode ?? 0) >= 500) {
    return l10n.errorServer;
  }
  return l10n.taskErrorGeneric;
}

/// Короткий хвост тела ошибки API (без сети).
String? taskErrorDetail(Object error) {
  if (error is TaskCancelledException) {
    return null;
  }
  if (error is! TaskApiException) {
    return null;
  }
  final m = error.message;
  if (m.isEmpty) {
    return null;
  }
  if (error.isNetworkTransportError) {
    return null;
  }
  const maxLen = 200;
  if (m.length <= maxLen) {
    return m;
  }
  var head = m.substring(0, maxLen);
  head = head.replaceAll(RegExp(r'(?:\.+|\u2026)+\s*$'), '');
  if (head.isEmpty) {
    head = m.substring(0, maxLen);
  }
  return '$head…';
}
