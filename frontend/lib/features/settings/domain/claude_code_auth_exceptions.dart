import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Sprint 15.B (F9, C1) — иерархия исключений OAuth-flow Claude Code.
/// Sprint 15.Major DRY: базируется на core/api/feature_exception.dart.
///
/// Device-flow выделяет «soft»-состояния (НЕ финальная ошибка):
///   - 202 authorization_pending — пользователь ещё не подтвердил;
///   - 429 slow_down            — поллим слишком часто;
///   - 410 expired_token / 410 access_denied — терминально.
abstract class ClaudeCodeAuthException extends FeatureException {
  ClaudeCodeAuthException(super.message, {super.originalError});
}

@immutable
class ClaudeCodeAuthCancelledException extends ClaudeCodeAuthException
    with MessageOnlyEquality {
  ClaudeCodeAuthCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class ClaudeCodeAuthorizationPendingException extends ClaudeCodeAuthException
    with MessageOnlyEquality {
  ClaudeCodeAuthorizationPendingException(String detail, {Object? originalError})
      : super(detail, originalError: originalError);
}

@immutable
class ClaudeCodeAuthSlowDownException extends ClaudeCodeAuthException
    with MessageOnlyEquality {
  ClaudeCodeAuthSlowDownException(String detail, {Object? originalError})
      : super(detail, originalError: originalError);
}

@immutable
class ClaudeCodeAuthFlowEndedException extends ClaudeCodeAuthException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ClaudeCodeAuthFlowEndedException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super(detail, originalError: originalError);
}

@immutable
class ClaudeCodeAuthOwnerMismatchException extends ClaudeCodeAuthException
    with MessageOnlyEquality {
  ClaudeCodeAuthOwnerMismatchException(String detail, {Object? originalError})
      : super(detail, originalError: originalError);
}

@immutable
class ClaudeCodeAuthForbiddenException extends ClaudeCodeAuthException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ClaudeCodeAuthForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class ClaudeCodeAuthNotFoundException extends ClaudeCodeAuthException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ClaudeCodeAuthNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class ClaudeCodeAuthConflictException extends ClaudeCodeAuthException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ClaudeCodeAuthConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);
}

@immutable
class ClaudeCodeAuthApiException extends ClaudeCodeAuthException
    with ApiCodeEquality {
  final int? statusCode;
  @override
  final String? apiErrorCode;

  ClaudeCodeAuthApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
    this.apiErrorCode,
  }) : super(detail, originalError: originalError);

  @override
  bool operator ==(Object other) =>
      super == other &&
      other is ClaudeCodeAuthApiException &&
      other.statusCode == statusCode;

  @override
  int get hashCode => Object.hash(super.hashCode, statusCode);
}

