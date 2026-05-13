import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Sprint 15.B (F9, C1) — иерархия исключений LLM-провайдеров.
/// Sprint 15.Major DRY: базируется на core/api/feature_exception.dart.
abstract class LLMProvidersException extends FeatureException {
  LLMProvidersException(super.message, {super.originalError});
}

@immutable
class LLMProvidersCancelledException extends LLMProvidersException
    with MessageOnlyEquality {
  LLMProvidersCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class LLMProvidersForbiddenException extends LLMProvidersException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  LLMProvidersForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class LLMProvidersNotFoundException extends LLMProvidersException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  LLMProvidersNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class LLMProvidersConflictException extends LLMProvidersException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  LLMProvidersConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);
}

@immutable
class LLMProvidersApiException extends LLMProvidersException
    with ApiCodeEquality {
  final int? statusCode;
  @override
  final String? apiErrorCode;

  LLMProvidersApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
    this.apiErrorCode,
  }) : super(detail, originalError: originalError);

  @override
  bool operator ==(Object other) =>
      super == other &&
      other is LLMProvidersApiException &&
      other.statusCode == statusCode;

  @override
  int get hashCode => Object.hash(super.hashCode, statusCode);
}

