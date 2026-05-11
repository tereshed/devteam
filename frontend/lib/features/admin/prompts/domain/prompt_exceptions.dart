import 'package:meta/meta.dart';

/// Базовый класс для ошибок [PromptsRepository].
abstract class PromptRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  PromptRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

@immutable
class PromptCancelledException extends PromptRepositoryException {
  PromptCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class PromptForbiddenException extends PromptRepositoryException {
  final String? apiErrorCode;

  PromptForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class PromptNotFoundException extends PromptRepositoryException {
  final String? apiErrorCode;

  PromptNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class PromptApiException extends PromptRepositoryException {
  final int? statusCode;

  PromptApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });
}
