import 'package:freezed_annotation/freezed_annotation.dart';

part 'onboarding_state.freezed.dart';

@freezed
abstract class OnboardingState with _$OnboardingState {
  const factory OnboardingState({
    @Default(false) bool hasLlmProviders,
    @Default(false) bool assistantConfigured,
    @Default(true) bool loading,
    @Default(false) bool hasError,
  }) = _OnboardingState;

  const OnboardingState._();

  bool get needsAssistantSetup =>
      !loading && !hasError && (!hasLlmProviders || !assistantConfigured);
}

@freezed
abstract class ProjectOnboardingState with _$ProjectOnboardingState {
  const factory ProjectOnboardingState({
    // Оркестратор — это Go-движок, отдельного LLM-агента нет. Готовность к
    // оркестрации определяется настройкой единственного LLM в петле — router.
    @Default(false) bool routerConfigured,
    @Default(true) bool loading,
    @Default(false) bool hasError,
  }) = _ProjectOnboardingState;

  const ProjectOnboardingState._();

  bool get needsAgentSetup => !loading && !hasError && !routerConfigured;
}
