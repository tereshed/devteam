import 'package:meta/meta.dart';

/// Базовый класс ошибок репозитория worktrees-debug API.
///
/// Эндпоинт `/worktrees` принадлежит подсистеме Orchestration v2, а не
/// Agents v2 — поэтому выделена своя иерархия исключений (нарушение
/// feature-first было бы маппить Worktrees-ошибки в `AgentV2*Exception`).
abstract class WorktreesRepositoryException implements Exception {
  final String message;
  final Object? originalError;

  WorktreesRepositoryException(this.message, {this.originalError});

  @override
  String toString() => message;
}

@immutable
class WorktreesCancelledException extends WorktreesRepositoryException {
  WorktreesCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class WorktreesForbiddenException extends WorktreesRepositoryException {
  final String? apiErrorCode;

  WorktreesForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class WorktreesNotFoundException extends WorktreesRepositoryException {
  final String? apiErrorCode;

  WorktreesNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class WorktreesApiException extends WorktreesRepositoryException {
  final int? statusCode;

  WorktreesApiException(
    super.message, {
    this.statusCode,
    super.originalError,
  });
}
