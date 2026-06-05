import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Базовое исключение слоя репозитория регулярных задач.
abstract class ScheduledTaskException extends FeatureException {
  ScheduledTaskException(super.message, {super.originalError});
}

@immutable
class ScheduledTaskCancelledException extends ScheduledTaskException
    with MessageOnlyEquality {
  ScheduledTaskCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class ScheduledTaskForbiddenException extends ScheduledTaskException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScheduledTaskForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class ScheduledTaskNotFoundException extends ScheduledTaskException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScheduledTaskNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Scheduled task not found: $detail', originalError: originalError);
}

@immutable
class ScheduledTaskValidationException extends ScheduledTaskException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  ScheduledTaskValidationException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Validation error: $detail', originalError: originalError);
}

@immutable
class ScheduledTaskApiException extends ScheduledTaskException
    with MessageOnlyEquality {
  final int? statusCode;

  ScheduledTaskApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
  }) : super('API error: $detail', originalError: originalError);
}
