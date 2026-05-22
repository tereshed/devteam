import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';
import 'package:frontend/features/onboarding/domain/onboarding_state.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'onboarding_providers.g.dart';

@riverpod
OnboardingState onboardingState(Ref ref) {
  final authState = ref.watch(authControllerProvider);

  return authState.maybeWhen(
    data: (user) {
      if (user == null) {
        return const OnboardingState(loading: false);
      }
      final isAdmin = user.role == 'admin';

      final myAgents = ref.watch(myAgentsListProvider);

      if (isAdmin) {
        final llmProviders = ref.watch(llmProvidersListProvider);
        if (myAgents.isLoading || llmProviders.isLoading) {
          return const OnboardingState(loading: true);
        }
        if (myAgents.hasError || llmProviders.hasError) {
          return const OnboardingState(loading: false, hasError: true);
        }
        final hasProviders =
            llmProviders.hasValue && llmProviders.value!.isNotEmpty;
        final agents = myAgents.hasValue ? myAgents.value : null;
        final assistant = agents?.items
            .where((a) => a.role == 'assistant')
            .firstOrNull;
        final configured = assistant != null &&
            assistant.model != null &&
            assistant.model!.isNotEmpty &&
            assistant.providerKind != null &&
            assistant.providerKind!.isNotEmpty;
        return OnboardingState(
          hasLlmProviders: hasProviders,
          assistantConfigured: configured,
          loading: false,
        );
      } else {
        if (myAgents.isLoading) {
          return const OnboardingState(loading: true);
        }
        if (myAgents.hasError) {
          return const OnboardingState(loading: false, hasError: true);
        }
        final agents = myAgents.hasValue ? myAgents.value : null;
        final assistant = agents?.items
            .where((a) => a.role == 'assistant')
            .firstOrNull;
        final configured = assistant != null &&
            assistant.model != null &&
            assistant.model!.isNotEmpty &&
            assistant.providerKind != null &&
            assistant.providerKind!.isNotEmpty;
        return OnboardingState(
          hasLlmProviders: true, // Non-admin doesn't need to configure LLM providers
          assistantConfigured: configured,
          loading: false,
        );
      }
    },
    loading: () => const OnboardingState(loading: true),
    orElse: () => const OnboardingState(loading: false, hasError: true),
  );
}

@riverpod
ProjectOnboardingState projectOnboardingState(
  Ref ref,
  String projectId,
) {
  final asyncTeam = ref.watch(teamProvider(projectId));

  if (asyncTeam.isLoading) {
    return const ProjectOnboardingState(loading: true);
  }

  if (asyncTeam.hasError || !asyncTeam.hasValue) {
    return const ProjectOnboardingState(loading: false, hasError: true);
  }

  final team = asyncTeam.requireValue;
  final agents = team.agents;

  bool isConfigured(String role) {
    final agent = agents.where((a) => a.role == role).firstOrNull;
    return agent != null &&
        agent.model != null &&
        agent.model!.isNotEmpty &&
        agent.providerKind != null &&
        agent.providerKind!.isNotEmpty;
  }

  return ProjectOnboardingState(
    orchestratorConfigured: isConfigured('orchestrator'),
    routerConfigured: isConfigured('router'),
    loading: false,
  );
}
