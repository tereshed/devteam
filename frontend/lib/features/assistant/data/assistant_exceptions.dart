import 'package:meta/meta.dart';

/// Базовый класс ошибок HTTP-репозитория ассистента (Sprint 21 §4 backend).
///
/// Подклассы с переопределённым `==` НЕ включают [originalError] в равенство
/// — поведение скопировано с `ConversationRepositoryException` (часто разные
/// ссылки на [DioException] при том же ответе API).
abstract class AssistantRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  AssistantRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Запрос отменён через [CancelToken] — не ошибка API.
@immutable
class AssistantCancelledException extends AssistantRepositoryException {
  AssistantCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantCancelledException && message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Сессия не найдена (404 на `/assistant/sessions/...`).
@immutable
class AssistantSessionNotFoundException extends AssistantRepositoryException {
  final String? apiErrorCode;

  AssistantSessionNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Assistant session not found: $detail',
            originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantSessionNotFoundException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Нет доступа к сессии (403).
@immutable
class AssistantForbiddenException extends AssistantRepositoryException {
  final String? apiErrorCode;

  AssistantForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantForbiddenException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// 409 Conflict с кодом `session_busy`. Несёт `pendingToolCallId`, если бэкенд
/// его передал — UI решает, показывать ли confirm-диалог по этому id.
@immutable
class AssistantSessionBusyException extends AssistantRepositoryException {
  final String? pendingToolCallId;

  AssistantSessionBusyException(
    String detail, {
    Object? originalError,
    this.pendingToolCallId,
  }) : super('Session busy: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantSessionBusyException &&
        message == other.message &&
        pendingToolCallId == other.pendingToolCallId;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, pendingToolCallId);
}

/// 409 `no_pending_confirmation` — confirm пришёл без активного pending.
@immutable
class AssistantNoPendingConfirmationException
    extends AssistantRepositoryException {
  AssistantNoPendingConfirmationException(
    String detail, {
    Object? originalError,
  }) : super('No pending confirmation: $detail',
            originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantNoPendingConfirmationException &&
        message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// 409 `already_confirmed` — параллельный confirm уже закрыл tool-row.
@immutable
class AssistantAlreadyConfirmedException extends AssistantRepositoryException {
  AssistantAlreadyConfirmedException(
    String detail, {
    Object? originalError,
  }) : super('Already confirmed: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantAlreadyConfirmedException &&
        message == other.message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// 429 Too Many Requests.
@immutable
class AssistantRateLimitedException extends AssistantRepositoryException {
  final String? apiErrorCode;

  AssistantRateLimitedException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Too many requests: $detail', originalError: originalError);

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantRateLimitedException &&
        message == other.message &&
        apiErrorCode == other.apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Прочие ошибки API. [apiErrorCode] — стабильное поле `error` из JSON.
@immutable
class AssistantApiException extends AssistantRepositoryException {
  final int? statusCode;
  final String? apiErrorCode;
  final bool isNetworkTransportError;

  AssistantApiException(
    super.message, {
    this.statusCode,
    this.apiErrorCode,
    super.originalError,
    this.isNetworkTransportError = false,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AssistantApiException &&
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
