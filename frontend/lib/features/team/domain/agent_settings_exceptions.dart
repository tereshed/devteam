import 'package:frontend/core/api/feature_exception.dart';
import 'package:meta/meta.dart';

/// Sprint 15.B (F9, C1) — иерархия исключений agent settings API.
/// Sprint 15.Major DRY: базируется на core/api/feature_exception.dart.
abstract class AgentSettingsException extends FeatureException {
  AgentSettingsException(super.message, {super.originalError});
}

@immutable
class AgentSettingsCancelledException extends AgentSettingsException
    with MessageOnlyEquality {
  AgentSettingsCancelledException(String detail, {Object? originalError})
      : super('Cancelled: $detail', originalError: originalError);
}

@immutable
class AgentSettingsForbiddenException extends AgentSettingsException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AgentSettingsForbiddenException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Forbidden: $detail', originalError: originalError);
}

@immutable
class AgentSettingsNotFoundException extends AgentSettingsException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AgentSettingsNotFoundException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Not found: $detail', originalError: originalError);
}

@immutable
class AgentSettingsConflictException extends AgentSettingsException
    with ApiCodeEquality {
  @override
  final String? apiErrorCode;

  AgentSettingsConflictException(
    String detail, {
    Object? originalError,
    this.apiErrorCode,
  }) : super('Conflict: $detail', originalError: originalError);
}

@immutable
class AgentSettingsApiException extends AgentSettingsException
    with ApiCodeEquality {
  final int? statusCode;
  @override
  final String? apiErrorCode;

  AgentSettingsApiException(
    String detail, {
    this.statusCode,
    Object? originalError,
    this.apiErrorCode,
  }) : super(detail, originalError: originalError);

  @override
  bool operator ==(Object other) =>
      super == other &&
      other is AgentSettingsApiException &&
      other.statusCode == statusCode;

  @override
  int get hashCode => Object.hash(super.hashCode, statusCode);
}

