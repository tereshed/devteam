import 'package:meta/meta.dart';

/// Базовый класс для ошибок репозитория проектов.
///
/// В подклассах с переопределённым `==` поле [originalError] **не** участвует в равенстве
/// (только диагностика; часто разные ссылки на [DioException] при том же ответе API).
abstract class ProjectRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  ProjectRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Запрос отменён ([CancelToken.cancel] и т.п.) — не ошибка API.
@immutable
class ProjectCancelledException extends ProjectRepositoryException {
  ProjectCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ProjectCancelledException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Нет прав на ресурс (403)
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class ProjectForbiddenException extends ProjectRepositoryException {
  final String? apiErrorCode;

  ProjectForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ProjectForbiddenException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Конфликт данных (409) — например, дубликат имени
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class ProjectConflictException extends ProjectRepositoryException {
  final String? apiErrorCode;

  ProjectConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ProjectConflictException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Общая ошибка API (4xx/5xx)
@immutable
class ProjectApiException extends ProjectRepositoryException {
  final int? statusCode;

  ProjectApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ProjectApiException &&
        message == other.message &&
        statusCode == other.statusCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, statusCode);
}
