import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/team/domain/agent_provider_rules.dart';

void main() {
  group('agentRoleUsesBackend', () {
    test('бекенд доступен всем, кроме orchestrator/router', () {
      for (final r in [
        'developer',
        'tester',
        'merger',
        'reviewer',
        'planner',
        'decomposer',
        'assistant',
      ]) {
        expect(agentRoleUsesBackend(r), isTrue, reason: r);
      }
    });
    test('orchestrator и router — без бекенда', () {
      for (final r in ['orchestrator', 'router']) {
        expect(agentRoleUsesBackend(r), isFalse, reason: r);
      }
    });
  });

  group('allowedProviderKindsForBackend', () {
    test('hermes — только anthropic/openrouter/hermes', () {
      expect(allowedProviderKindsForBackend('hermes'), {'anthropic', 'openrouter', 'hermes'});
    });
    test('antigravity — только antigravity*', () {
      expect(allowedProviderKindsForBackend('antigravity'),
          {'antigravity', 'antigravity_oauth'});
    });
    test('claude-code/aider/custom/null — любой провайдер', () {
      for (final b in ['claude-code', 'aider', 'custom', null]) {
        final allowed = allowedProviderKindsForBackend(b);
        expect(allowed.contains('anthropic'), isTrue, reason: '$b');
        expect(allowed.contains('deepseek'), isTrue, reason: '$b');
        expect(allowed.contains('antigravity'), isTrue, reason: '$b');
      }
    });
  });

  group('backendRequiresProvider', () {
    test('только hermes требует провайдера', () {
      expect(backendRequiresProvider('hermes'), isTrue);
      for (final b in ['claude-code', 'aider', 'custom', 'antigravity', null]) {
        expect(backendRequiresProvider(b), isFalse, reason: '$b');
      }
    });
  });

  group('integrationToAgentProviderKind', () {
    test('OAuth-интеграции мапятся в *_oauth', () {
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.claudeCodeOAuth),
          'anthropic_oauth');
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.antigravityOAuth),
          'antigravity_oauth');
    });
    test('Anthropic-совместимые — идентично', () {
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.anthropic),
          'anthropic');
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.deepseek),
          'deepseek');
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.openrouter),
          'openrouter');
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.hermes),
          'hermes');
    });
    test('openai/gemini/qwen нельзя использовать для агента (null)', () {
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.openai), isNull);
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.gemini), isNull);
      expect(integrationToAgentProviderKind(LlmIntegrationProvider.qwen), isNull);
    });
  });

  group('configuredAgentProviderKinds', () {
    LlmProviderConnection conn(
      LlmIntegrationProvider p,
      LlmProviderConnectionStatus s,
    ) =>
        LlmProviderConnection(provider: p, status: s);

    test('берёт только connected, маппит и дедуплицирует; openai отбрасывает', () {
      final result = configuredAgentProviderKinds([
        conn(LlmIntegrationProvider.anthropic, LlmProviderConnectionStatus.connected),
        conn(LlmIntegrationProvider.deepseek, LlmProviderConnectionStatus.connected),
        conn(LlmIntegrationProvider.openrouter, LlmProviderConnectionStatus.disconnected),
        conn(LlmIntegrationProvider.openai, LlmProviderConnectionStatus.connected),
        conn(LlmIntegrationProvider.claudeCodeOAuth, LlmProviderConnectionStatus.connected),
      ]);
      expect(result, containsAll(['anthropic', 'deepseek', 'anthropic_oauth']));
      expect(result.contains('openrouter'), isFalse, reason: 'disconnected');
      expect(result.contains('openai'), isFalse, reason: 'не usable для агента');
    });
  });
}
