import 'package:meta/meta.dart';

/// Базовый класс ошибок read-only Orchestration v2 API
/// (artifacts / router-decisions). Отдельная иерархия, чтобы не тащить
/// `AgentV2*Exception` в чужой домен — это нарушение feature-first.
abstract class OrchestrationV2RepositoryException implements Exception {
  final String message;
  final Object? originalError;

  OrchestrationV2RepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

@immutable
class OrchestrationV2CancelledException
    extends OrchestrationV2RepositoryException {
  OrchestrationV2CancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class OrchestrationV2ForbiddenException
    extends OrchestrationV2RepositoryException {
  final String? apiErrorCode;

  OrchestrationV2ForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class OrchestrationV2NotFoundException
    extends OrchestrationV2RepositoryException {
  final String? apiErrorCode;

  OrchestrationV2NotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class OrchestrationV2ApiException extends OrchestrationV2RepositoryException {
  final int? statusCode;

  OrchestrationV2ApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });
}
