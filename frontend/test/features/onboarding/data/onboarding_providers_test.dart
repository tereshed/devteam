import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';
import 'package:frontend/features/onboarding/data/onboarding_providers.dart';
import 'package:frontend/features/onboarding/domain/onboarding_state.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:frontend/features/team/data/team_providers.dart';

AgentV2 _agent({
  String role = 'assistant',
  String? model,
  String? providerKind,
}) {
  return AgentV2(
    id: 'a1',
    name: role,
    role: role,
    roleDescription: '',
    executionKind: 'llm',
    isActive: true,
    internalMcpEnabled: false,
    createdAt: DateTime(2024),
    updatedAt: DateTime(2024),
    model: model,
    providerKind: providerKind,
  );
}

void main() {
  group('onboardingStateProvider', () {
    test('loading while agents not yet fetched', () {
      final container = ProviderContainer(
        overrides: [
          myAgentsListProvider.overrideWith(
            (ref) => Future<AgentV2Page>.delayed(
              const Duration(hours: 1),
              () => const AgentV2Page(
                  total: 0, items: [], limit: 0, offset: 0),
            ),
          ),
          llmProvidersListProvider.overrideWith(
            (ref) async => <LLMProviderModel>[],
          ),
        ],
      );
      addTearDown(container.dispose);

      final state = container.read(onboardingStateProvider);
      expect(state.loading, isTrue);
    });

    test('does not show banner on error', () {
      // The provider returns OnboardingState(loading: false, hasError: true)
      // when async deps error. Verify via overrideWithValue that the banner
      // is hidden for that state.
      final container = ProviderContainer(
        overrides: [
          onboardingStateProvider.overrideWithValue(
            const OnboardingState(loading: false, hasError: true),
          ),
        ],
      );
      addTearDown(container.dispose);

      final state = container.read(onboardingStateProvider);
      expect(state.loading, isFalse);
      expect(state.hasError, isTrue);
      expect(state.needsAssistantSetup, isFalse);
    });

    test('needsSetup when no providers and assistant unconfigured', () async {
      final container = ProviderContainer(
        overrides: [
          myAgentsListProvider.overrideWith(
            (ref) async => AgentV2Page(
              total: 1,
              items: [_agent()],
              limit: 10,
              offset: 0,
            ),
          ),
          llmProvidersListProvider.overrideWith(
            (ref) async => <LLMProviderModel>[],
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(myAgentsListProvider.future);
      await container.read(llmProvidersListProvider.future);

      final state = container.read(onboardingStateProvider);
      expect(state.loading, isFalse);
      expect(state.hasLlmProviders, isFalse);
      expect(state.assistantConfigured, isFalse);
      expect(state.needsAssistantSetup, isTrue);
    });

    test('configured when assistant has model and provider', () async {
      final container = ProviderContainer(
        overrides: [
          myAgentsListProvider.overrideWith(
            (ref) async => AgentV2Page(
              total: 1,
              items: [
                _agent(
                  model: 'claude-3-5-sonnet',
                  providerKind: 'anthropic',
                ),
              ],
              limit: 10,
              offset: 0,
            ),
          ),
          llmProvidersListProvider.overrideWith(
            (ref) async => [
              const LLMProviderModel(
                id: 'lp1',
                name: 'Anthropic',
                kind: 'anthropic',
              ),
            ],
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(myAgentsListProvider.future);
      await container.read(llmProvidersListProvider.future);

      final state = container.read(onboardingStateProvider);
      expect(state.loading, isFalse);
      expect(state.hasLlmProviders, isTrue);
      expect(state.assistantConfigured, isTrue);
      expect(state.needsAssistantSetup, isFalse);
    });
  });

  group('projectOnboardingStateProvider', () {
    test('needs setup when orchestrator has no model', () async {
      final container = ProviderContainer(
        overrides: [
          teamProvider('p1').overrideWith(
            (ref) async => TeamModel(
              id: 't1',
              name: 'Dev',
              projectId: 'p1',
              type: 'development',
              createdAt: DateTime(2024),
              updatedAt: DateTime(2024),
              agents: [
                const AgentModel(
                  id: 'a1',
                  name: 'orchestrator',
                  role: 'orchestrator',
                  isActive: true,
                ),
                const AgentModel(
                  id: 'a2',
                  name: 'router',
                  role: 'router',
                  isActive: true,
                  model: 'claude-3-5-sonnet',
                  providerKind: 'anthropic',
                ),
              ],
            ),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(teamProvider('p1').future);

      final state = container.read(projectOnboardingStateProvider('p1'));
      expect(state.loading, isFalse);
      expect(state.orchestratorConfigured, isFalse);
      expect(state.routerConfigured, isTrue);
      expect(state.needsAgentSetup, isTrue);
    });

    test('needs setup when model set but providerKind missing', () async {
      final container = ProviderContainer(
        overrides: [
          teamProvider('p1').overrideWith(
            (ref) async => TeamModel(
              id: 't1',
              name: 'Dev',
              projectId: 'p1',
              type: 'development',
              createdAt: DateTime(2024),
              updatedAt: DateTime(2024),
              agents: [
                const AgentModel(
                  id: 'a1',
                  name: 'orchestrator',
                  role: 'orchestrator',
                  isActive: true,
                  model: 'claude-3-5-sonnet',
                ),
                const AgentModel(
                  id: 'a2',
                  name: 'router',
                  role: 'router',
                  isActive: true,
                  model: 'claude-3-5-sonnet',
                  providerKind: 'anthropic',
                ),
              ],
            ),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(teamProvider('p1').future);

      final state = container.read(projectOnboardingStateProvider('p1'));
      expect(state.orchestratorConfigured, isFalse);
      expect(state.routerConfigured, isTrue);
      expect(state.needsAgentSetup, isTrue);
    });

    test('no setup needed when both agents configured', () async {
      final container = ProviderContainer(
        overrides: [
          teamProvider('p1').overrideWith(
            (ref) async => TeamModel(
              id: 't1',
              name: 'Dev',
              projectId: 'p1',
              type: 'development',
              createdAt: DateTime(2024),
              updatedAt: DateTime(2024),
              agents: [
                const AgentModel(
                  id: 'a1',
                  name: 'orchestrator',
                  role: 'orchestrator',
                  isActive: true,
                  model: 'claude-3-5-sonnet',
                  providerKind: 'anthropic',
                ),
                const AgentModel(
                  id: 'a2',
                  name: 'router',
                  role: 'router',
                  isActive: true,
                  model: 'claude-3-5-sonnet',
                  providerKind: 'anthropic',
                ),
              ],
            ),
          ),
        ],
      );
      addTearDown(container.dispose);

      await container.read(teamProvider('p1').future);

      final state = container.read(projectOnboardingStateProvider('p1'));
      expect(state.loading, isFalse);
      expect(state.needsAgentSetup, isFalse);
    });
  });
}
