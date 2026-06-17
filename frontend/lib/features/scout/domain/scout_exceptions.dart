import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Базовое исключение слоя репозитория разведчика.
abstract class ScoutException extends FeatureException {
  ScoutException(super.message, {super.originalError});
}

@immutable
class ScoutCancelledException extends ScoutException with MessageOnlyEquality {
  ScoutCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class ScoutForbiddenException extends ScoutException with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScoutForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class ScoutNotFoundException extends ScoutException with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScoutNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class ScoutValidationException extends ScoutException with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScoutValidationException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Validation error: $detail', originalError: originalError);
}

@immutable
class ScoutApiException extends ScoutException with MessageOnlyEquality {
  final int? statusCode;

  ScoutApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
  }) : super('API error: $detail', originalError: originalError);
}
