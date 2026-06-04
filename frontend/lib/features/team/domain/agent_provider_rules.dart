import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

/// Правила валидных комбинаций для настройки агента — SSOT с бэком
/// (`sandbox_auth_resolver.go`, `agent_service.go`, миграции 031/038).
///
/// Цель — не дать совершить ошибку в форме: показывать только подключённые
/// провайдеры, ограничивать бекенд ролью, а выбор провайдера — бекендом.

/// Бекенд НЕ нужен только чистым LLM-диспетчерам: router (решает, кого звать)
/// и orchestrator (корень цикла). Всем остальным ролям (planner, decomposer,
/// reviewer, developer, tester, merger) code_backend доступен — у них есть и
/// модель, и бекенд одновременно.
const Set<String> kNoBackendRoles = {'orchestrator', 'router'};

/// `true`, если роль выбирает code_backend (все, кроме orchestrator/router).
bool agentRoleUsesBackend(String role) => !kNoBackendRoles.contains(role);

/// Системные роли (зеркало backend `AgentRole.IsSystem`): на них завязана механика
/// оркестрации (branch-policy, дефолтные агенты, исключение assistant из каталога
/// Router'а). Менять такую роль нельзя — read-only. Кастомные роли редактируемы.
const Set<String> kSystemAgentRoles = {
  'worker', 'supervisor', 'orchestrator', 'planner', 'developer', 'reviewer',
  'tester', 'devops', 'router', 'decomposer', 'merger', 'assistant',
};

/// `true`, если роль кастомная (не системная) — её можно редактировать в UI.
bool agentRoleIsCustom(String role) => !kSystemAgentRoles.contains(role);

const Set<String> _allProviderKinds = {
  'anthropic',
  'anthropic_oauth',
  'deepseek',
  'zhipu',
  'openrouter',
  'antigravity',
  'antigravity_oauth',
  'hermes',
};

/// Допустимые `provider_kind` для выбранного бекенда (см. sandbox_auth_resolver):
/// claude-code / aider / custom — любой провайдер (резолвер ставит нужный env);
/// hermes — только anthropic / openrouter (для остальных env пустой → падение);
/// antigravity-бекенд — только antigravity / antigravity_oauth.
Set<String> allowedProviderKindsForBackend(String? backend) => switch (backend) {
      'hermes' => const {'anthropic', 'openrouter', 'hermes'},
      'antigravity' => const {'antigravity', 'antigravity_oauth'},
      _ => _allProviderKinds, // claude-code, aider, custom, null
    };

/// `hermes` обязательно требует явный `provider_kind` (нет fallback на дефолт).
bool backendRequiresProvider(String? backend) => backend == 'hermes';

/// Интеграция (экран LLM Integrations) → `provider_kind` агента.
/// `null` — провайдера нельзя использовать для агента (openai/gemini/qwen —
/// не Anthropic-совместимы, нет резолвера в sandbox_auth_resolver).
String? integrationToAgentProviderKind(LlmIntegrationProvider p) => switch (p) {
      LlmIntegrationProvider.claudeCodeOAuth => 'anthropic_oauth',
      LlmIntegrationProvider.antigravityOAuth => 'antigravity_oauth',
      LlmIntegrationProvider.antigravity => 'antigravity',
      LlmIntegrationProvider.anthropic => 'anthropic',
      LlmIntegrationProvider.deepseek => 'deepseek',
      LlmIntegrationProvider.openrouter => 'openrouter',
      LlmIntegrationProvider.zhipu => 'zhipu',
      LlmIntegrationProvider.openai => null,
      LlmIntegrationProvider.gemini => null,
      LlmIntegrationProvider.qwen => null,
      LlmIntegrationProvider.hermes => 'hermes',
    };

/// Подключённые провайдеры (status=connected), отображённые в `provider_kind`
/// агента и пригодные для выбора. Дедуплицировано, порядок стабильный.
List<String> configuredAgentProviderKinds(
  Iterable<LlmProviderConnection> connections,
) {
  final out = <String>[];
  for (final c in connections) {
    if (c.status != LlmProviderConnectionStatus.connected) {
      continue;
    }
    final kind = integrationToAgentProviderKind(c.provider);
    if (kind != null && !out.contains(kind)) {
      out.add(kind);
    }
  }
  return out;
}
