import 'package:meta/meta.dart';

/// Корень иерархии ошибок HTTP/API на фронтенде (не привязан к конкретной фиче).
///
/// В подклассах с переопределённым `==` поле [originalError] **не** участвует в равенстве
/// (только диагностика; часто разные ссылки на [DioException] при том же ответе API).
abstract class ApiException implements Exception {
  final String message;
  final Object? originalError;

  ApiException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Не авторизован (401). Матчить по [statusCode], не по полю `error` в JSON.
///
/// [apiErrorCode] — стабильное поле `error` из JSON (например `access_denied`), если есть.
@immutable
class UnauthorizedException extends ApiException {
  final String? apiErrorCode;

  UnauthorizedException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Unauthorized: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is UnauthorizedException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Проект не найден (404 на эндпоинтах проекта).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class ProjectNotFoundException extends ApiException {
  final String? apiErrorCode;

  ProjectNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Project not found: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ProjectNotFoundException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}
