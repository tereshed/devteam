import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Sprint 15.Major3 DRY — base мигрирован на FeatureException.
/// Внешний API классов сохранён (TeamRepositoryException + 6 subclasses).
abstract class TeamRepositoryException extends FeatureException {
  TeamRepositoryException(super.message, {super.originalError});
}

@immutable
class TeamCancelledException extends TeamRepositoryException
    with MessageOnlyEquality {
  TeamCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class TeamForbiddenException extends TeamRepositoryException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  TeamForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class TeamNotFoundException extends TeamRepositoryException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  TeamNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Team not found: $detail', originalError: originalError);
}

@immutable
class TeamConflictException extends TeamRepositoryException
    with MessageOnlyEquality {
  TeamConflictException(String detail, {Object? originalError})
      : super('Conflict: $detail', originalError: originalError);
}

@immutable
class TeamApiException extends TeamRepositoryException {
  final int? statusCode;

  TeamApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is TeamApiException &&
        message == other.message &&
        statusCode == other.statusCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, statusCode);
}

@immutable
class TeamProjectMismatchException extends TeamRepositoryException
    with MessageOnlyEquality {
  TeamProjectMismatchException(String detail, {Object? originalError})
      : super('Team project mismatch: $detail', originalError: originalError);
}
