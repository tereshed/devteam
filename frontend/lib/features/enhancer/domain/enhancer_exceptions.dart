import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Базовое исключение слоя репозитория энхансера.
abstract class EnhancerException extends FeatureException {
  EnhancerException(super.message, {super.originalError});
}

@immutable
class EnhancerCancelledException extends EnhancerException
    with MessageOnlyEquality {
  EnhancerCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class EnhancerForbiddenException extends EnhancerException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  EnhancerForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class EnhancerNotFoundException extends EnhancerException with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  EnhancerNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class EnhancerValidationException extends EnhancerException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  EnhancerValidationException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Validation error: $detail', originalError: originalError);
}

/// 409 — прогон уже выполняется.
@immutable
class EnhancerRunInProgressException extends EnhancerException
    with MessageOnlyEquality {
  EnhancerRunInProgressException(String detail, {Object? originalError})
      : super('Run in progress: $detail', originalError: originalError);
}

@immutable
class EnhancerApiException extends EnhancerException with MessageOnlyEquality {
  final int? statusCode;

  EnhancerApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
  }) : super('API error: $detail', originalError: originalError);
}
