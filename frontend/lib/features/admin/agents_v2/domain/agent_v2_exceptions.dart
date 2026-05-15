import 'package:meta/meta.dart';

abstract class AgentV2RepositoryException implements Exception {
  final String message;
  final Object? originalError;

  AgentV2RepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

@immutable
class AgentV2CancelledException extends AgentV2RepositoryException {
  AgentV2CancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class AgentV2ForbiddenException extends AgentV2RepositoryException {
  final String? apiErrorCode;

  AgentV2ForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class AgentV2NotFoundException extends AgentV2RepositoryException {
  final String? apiErrorCode;

  AgentV2NotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class AgentV2ConflictException extends AgentV2RepositoryException {
  final String? apiErrorCode;

  AgentV2ConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);
}

@immutable
class AgentV2ApiException extends AgentV2RepositoryException {
  final int? statusCode;

  AgentV2ApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });
}
