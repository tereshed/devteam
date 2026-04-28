/// Базовый класс для ошибок репозитория проектов
abstract class ProjectRepositoryException implements Exception {
  final String message;
  ProjectRepositoryException(this.message);

  @override
  String toString() => message;
}

/// Проект не найден (404)
class ProjectNotFoundException extends ProjectRepositoryException {
  ProjectNotFoundException(String message) : super('Project not found: $message');
}

/// Не авторизован (401)
/// TODO(10.X): вынести в lib/core/api/api_exceptions.dart, единый для всех репозиториев.
/// AuthRepository кидает InvalidCredentialsException на 401 — нужна унификация.
class UnauthorizedException extends ProjectRepositoryException {
  UnauthorizedException(String message) : super('Unauthorized: $message');
}

/// Нет прав на ресурс (403)
class ProjectForbiddenException extends ProjectRepositoryException {
  ProjectForbiddenException(String message) : super('Forbidden: $message');
}

/// Конфликт данных (409) — например, дубликат имени
class ProjectConflictException extends ProjectRepositoryException {
  ProjectConflictException(String message) : super('Conflict: $message');
}

/// Общая ошибка API (4xx/5xx)
class ProjectApiException extends ProjectRepositoryException {
  final int? statusCode;
  final Object? originalError;

  ProjectApiException(
    String message, {
    this.statusCode,
    this.originalError,
  }) : super(message);
}
