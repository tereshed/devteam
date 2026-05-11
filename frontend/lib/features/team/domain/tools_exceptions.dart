import 'package:meta/meta.dart';

/// Базовый класс для ошибок [ToolsRepository].
abstract class ToolsRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  ToolsRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

@immutable
class ToolsCancelledException extends ToolsRepositoryException {
  ToolsCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class ToolsApiException extends ToolsRepositoryException {
  final int? statusCode;

  ToolsApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });
}
