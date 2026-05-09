import 'package:meta/meta.dart';

/// Базовый класс ошибок HTTP-репозитория задач.
///
/// В подклассах с переопределённым `==` поле [originalError] **не** участвует в равенстве.
abstract class TaskRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  TaskRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Запрос отменён ([CancelToken.cancel] и т.п.).
@immutable
class TaskCancelledException extends TaskRepositoryException {
  TaskCancelledException(super.detail, {super.originalError});

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskCancelledException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Задача не найдена (404 на `/tasks/...`, включая вложенные пути вроде `…/messages`).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class TaskNotFoundException extends TaskRepositoryException {
  final String? apiErrorCode;

  TaskNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Task not found: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskNotFoundException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Нет доступа к операции с задачей (403 на любых путях задач, включая `/projects/{id}/tasks`).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class TaskForbiddenException extends TaskRepositoryException {
  final String? apiErrorCode;

  TaskForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskForbiddenException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Конфликт состояния задачи (409).
///
/// [apiErrorCode] — стабильное поле `error` из JSON (часто `conflict`), если есть.
@immutable
class TaskConflictException extends TaskRepositoryException {
  final String? apiErrorCode;

  TaskConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskConflictException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Невозможно обработать запрос (422).
///
/// [apiErrorCode] — стабильное поле `error` из JSON (часто `unprocessable`), если есть.
@immutable
class TaskUnprocessableException extends TaskRepositoryException {
  final String? apiErrorCode;

  TaskUnprocessableException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Unprocessable: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskUnprocessableException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Прочие ошибки API и транспорт без HTTP ([parseDioApiError]).
///
/// [apiErrorCode] — стабильное поле `error` из JSON; не использовать [message] как ключ логики UI.
@immutable
class TaskApiException extends TaskRepositoryException {
  final int? statusCode;
  final String? apiErrorCode;

  /// Сеть без HTTP-ответа (таймаут / connection error).
  final bool isNetworkTransportError;

  TaskApiException(
    super.message, {
    this.statusCode,
    this.apiErrorCode,
    super.originalError,
    this.isNetworkTransportError = false,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TaskApiException &&
        message == other.message &&
        statusCode == other.statusCode &&
        apiErrorCode == other.apiErrorCode &&
        isNetworkTransportError == other.isNetworkTransportError;
  }

  @override
  int get hashCode => Object.hash(
        runtimeType,
        message,
        statusCode,
        apiErrorCode,
        isNetworkTransportError,
      );
}

/// Задача принадлежит другому проекту, чем сегмент URL (`projectId` в дашборде).
///
/// UI (12.5): показывать [AppLocalizations.taskDetailProjectMismatch], без сырого текста ошибки.
@immutable
class TaskDetailProjectMismatchException implements Exception {
  TaskDetailProjectMismatchException({
    required this.taskId,
    required this.expectedProjectId,
    required this.actualProjectId,
  });

  final String taskId;
  final String expectedProjectId;
  final String actualProjectId;

  @override
  String toString() =>
      'TaskDetailProjectMismatchException(taskId=$taskId, '
      'expectedProjectId=$expectedProjectId, actualProjectId=$actualProjectId)';
}
