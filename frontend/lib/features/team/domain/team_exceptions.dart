import 'package:meta/meta.dart';

/// Базовый класс для ошибок [TeamRepository].
///
/// В подклассах с переопределённым `==` поле [originalError] **не** участвует в равенстве.
abstract class TeamRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  TeamRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Запрос отменён ([CancelToken.cancel] и т.п.).
@immutable
class TeamCancelledException extends TeamRepositoryException {
  TeamCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TeamCancelledException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Нет прав (403).
@immutable
class TeamForbiddenException extends TeamRepositoryException {
  final String? apiErrorCode;

  TeamForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TeamForbiddenException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Команда не найдена (404).
@immutable
class TeamNotFoundException extends TeamRepositoryException {
  final String? apiErrorCode;

  TeamNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Team not found: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TeamNotFoundException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Общая ошибка API (4xx/5xx).
@immutable
class TeamApiException extends TeamRepositoryException {
  final int? statusCode;

  TeamApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TeamApiException &&
        message == other.message &&
        statusCode == other.statusCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, statusCode);
}

/// Ответ API: [TeamModel.projectId] не совпадает с запрошенным маршрутом.
@immutable
class TeamProjectMismatchException extends TeamRepositoryException {
  TeamProjectMismatchException(String detail, {Object? originalError})
      : super('Team project mismatch: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is TeamProjectMismatchException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}
