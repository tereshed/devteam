import 'package:meta/meta.dart';

/// Sprint 15.Major DRY — общий базовый класс для feature-exceptions.
///
/// Канон Sprint 13 повторялся 4× для Team/AgentSettings/ClaudeCodeAuth/LLMProviders.
/// Этот класс выносит boilerplate (message + originalError + toString) в одно место,
/// а подклассы добавляют только специфичные поля (apiErrorCode, statusCode).
///
/// `originalError` НЕ участвует в `==` (разные Dio-инстансы при одинаковой реакции API).
abstract class FeatureException implements Exception {
  final String message;
  final Object? originalError;

  FeatureException(this.message, {this.originalError});

  @override
  String toString() => message;
}

/// Подмешать к подклассу: общий `==`/`hashCode` по runtimeType+message+apiErrorCode.
mixin ApiCodeEquality on FeatureException {
  String? get apiErrorCode;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    if (other.runtimeType != runtimeType) return false;
    return other is FeatureException &&
        message == other.message &&
        (other as ApiCodeEquality).apiErrorCode == apiErrorCode;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message, apiErrorCode);
}

/// Подмешать к «cancelled»/«mismatch»/«simple» подклассам — `==` только по runtimeType+message.
mixin MessageOnlyEquality on FeatureException {
  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other.runtimeType == runtimeType &&
        other is FeatureException &&
        other.message == message;
  }

  @override
  int get hashCode => Object.hash(runtimeType, message);
}

/// Универсальный Cancelled-класс (`runtimeType`+`message` equality).
/// Используется feature-классами через `extends FeatureCancelledException`.
///
/// Sprint 15.Major4: убирает дублирование 3× `_CancelEq` mixin в claude_code_auth /
/// llm_providers / agent_settings exception-файлах.
@immutable
class FeatureCancelledException extends FeatureException
    with MessageOnlyEquality {
  FeatureCancelledException(super.message, {super.originalError});
}
