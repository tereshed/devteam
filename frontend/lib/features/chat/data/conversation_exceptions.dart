import 'package:meta/meta.dart';

/// Базовый класс ошибок HTTP-репозитория чатов.
///
/// В подклассах с переопределённым `==` поле [originalError] **не** участвует в равенстве
/// (только диагностика; часто разные ссылки на [DioException] при том же ответе API).
abstract class ConversationRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  ConversationRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Запрос отменён ([CancelToken.cancel] и т.п.) — не ошибка API.
@immutable
class ConversationCancelledException extends ConversationRepositoryException {
  ConversationCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ConversationCancelledException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Чат не найден (404 на `/conversations/...`).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class ConversationNotFoundException extends ConversationRepositoryException {
  final String? apiErrorCode;

  ConversationNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conversation not found: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ConversationNotFoundException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Нет доступа к чату (403).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть.
@immutable
class ConversationForbiddenException extends ConversationRepositoryException {
  final String? apiErrorCode;

  ConversationForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ConversationForbiddenException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Слишком много запросов (429).
///
/// [apiErrorCode] — стабильное поле `error` из JSON (например `too_many_requests`), если есть.
@immutable
class ConversationRateLimitedException extends ConversationRepositoryException {
  final String? apiErrorCode;

  ConversationRateLimitedException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Too many requests: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) {
      return true;
    }
    return other is ConversationRateLimitedException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Прочие ошибки API (валидация, сервер, внешние сервисы).
///
/// [apiErrorCode] — стабильное поле `error` из JSON, если есть; не использовать [message] как ключ логики.
@immutable
class ConversationApiException extends ConversationRepositoryException {
  final int? statusCode;
  final String? apiErrorCode;

  /// Сеть без HTTP-ответа (таймаут / connection error), см. [parseDioApiError].
  final bool isNetworkTransportError;

  ConversationApiException(
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
    return other is ConversationApiException &&
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
