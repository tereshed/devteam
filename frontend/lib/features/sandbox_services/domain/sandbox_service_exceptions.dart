import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Базовое исключение слоя репозитория сервис-сайдкаров.
abstract class SandboxServiceException extends FeatureException {
  SandboxServiceException(super.message, {super.originalError});
}

@immutable
class SandboxServiceCancelledException extends SandboxServiceException
    with MessageOnlyEquality {
  SandboxServiceCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class SandboxServiceForbiddenException extends SandboxServiceException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  SandboxServiceForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class SandboxServiceNotFoundException extends SandboxServiceException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  SandboxServiceNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class SandboxServiceValidationException extends SandboxServiceException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  SandboxServiceValidationException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Validation error: $detail', originalError: originalError);
}

@immutable
class SandboxServiceApiException extends SandboxServiceException
    with MessageOnlyEquality {
  final int? statusCode;

  SandboxServiceApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
  }) : super('API error: $detail', originalError: originalError);
}
