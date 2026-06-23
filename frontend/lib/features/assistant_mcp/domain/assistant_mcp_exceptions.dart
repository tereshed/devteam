import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Базовое исключение слоя репозитория MCP-серверов ассистента.
abstract class AssistantMcpException extends FeatureException {
  AssistantMcpException(super.message, {super.originalError});
}

@immutable
class AssistantMcpCancelledException extends AssistantMcpException
    with MessageOnlyEquality {
  AssistantMcpCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class AssistantMcpForbiddenException extends AssistantMcpException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AssistantMcpForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class AssistantMcpNotFoundException extends AssistantMcpException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AssistantMcpNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class AssistantMcpValidationException extends AssistantMcpException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AssistantMcpValidationException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Validation error: $detail', originalError: originalError);
}

@immutable
class AssistantMcpApiException extends AssistantMcpException
    with MessageOnlyEquality {
  final int? statusCode;

  AssistantMcpApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
  }) : super('API error: $detail', originalError: originalError);
}
