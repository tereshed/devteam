import 'package:freezed_annotation/freezed_annotation.dart';

part 'agent_settings_model.freezed.dart';
part 'agent_settings_model.g.dart';

/// Sprint 15.28 — настройки per-agent (GET /agents/:id/settings).
///
/// `codeBackendSettings` и `sandboxPermissions` приходят как сырые JSON-объекты
/// (Map<String, dynamic>) — формат остаётся гибким, чтобы UI мог разбирать вкладки
/// (модель/провайдер, MCP, Skills, Разрешения) самостоятельно.
@freezed
abstract class AgentSettingsModel with _$AgentSettingsModel {
  const factory AgentSettingsModel({
    @JsonKey(name: 'agent_id') required String agentID,
    @JsonKey(name: 'code_backend') String? codeBackend,
    @JsonKey(name: 'code_backend_settings')
    @Default(<String, dynamic>{})
    Map<String, dynamic> codeBackendSettings,
    @JsonKey(name: 'sandbox_permissions')
    @Default(<String, dynamic>{})
    Map<String, dynamic> sandboxPermissions,
  }) = _AgentSettingsModel;

  factory AgentSettingsModel.fromJson(Map<String, dynamic> json) =>
      _$AgentSettingsModelFromJson(json);
}

/// Все допустимые значения CodeBackend на бэке (sync c models.CodeBackend).
/// Sprint 15.e2e: `claude-code-via-proxy` удалён — не-Anthropic провайдеры
/// теперь работают через `claude-code` + agent.provider_kind (native endpoint).
/// Sprint 16: добавлен `hermes` — Hermes Agent (Nous Research, MIT). Своя
/// схема env (OPENROUTER_API_KEY и т.п.), своя docker-image
/// devteam/sandbox-hermes; provider_kind агента маппится в env через
/// AgentProviderKind.HermesEnvVar.
const List<String> kSupportedCodeBackends = [
  'claude-code',
  'aider',
  'hermes',
  'custom',
];

/// Все допустимые значения agent.provider_kind на бэке (sync c models.AgentProviderKind).
/// Sprint 15.e2e — связывает агента с per-user креденшалами.
const List<String> kSupportedAgentProviderKinds = [
  'anthropic',
  'anthropic_oauth',
  'deepseek',
  'zhipu',
  'openrouter',
];

/// Допустимые значения permissions.defaultMode (Claude Code CLI).
const List<String> kSupportedPermissionModes = [
  'default',
  'acceptEdits',
  'plan',
  'bypassPermissions',
];

/// Sprint 16.C — допустимые `permission_mode` для Hermes.
/// Backend отклоняет `plan`/`default` на PUT /agents/{id}/settings (400),
/// поэтому здесь их сознательно нет — UI не должен предлагать.
const List<String> kHermesPermissionModes = ['yolo', 'accept'];

/// Sprint 16.C — каталог Hermes toolsets (зеркало backend.HermesToolsetCatalog).
/// При расширении (новые toolsets в hermes) обновлять обе локации.
const List<String> kHermesToolsetCatalog = [
  'file_ops',
  'shell',
  'web_fetch',
  'web_search',
  'code_review',
  'todo',
];

/// Sprint 16.C — дефолтные значения Hermes-настроек (синхронизированы с
/// service.hermesDefaults() в Go-бэке). Если значения расходятся — UI
/// будет показывать другие defaults, чем backend применит для пустых полей.
class HermesDefaults {
  static const List<String> toolsets = ['file_ops', 'shell'];
  static const String permissionMode = 'yolo';
  static const int maxTurns = 12;
}

/// MCP-сервер из реестра mcp_servers_registry.
@freezed
abstract class MCPServerRegistryModel with _$MCPServerRegistryModel {
  const factory MCPServerRegistryModel({
    required String id,
    required String name,
    @Default('') String description,
    required String transport,
    @Default('') String command,
    @Default('') String url,
    @Default('global') String scope,
    @JsonKey(name: 'is_active') @Default(true) bool isActive,
  }) = _MCPServerRegistryModel;

  factory MCPServerRegistryModel.fromJson(Map<String, dynamic> json) =>
      _$MCPServerRegistryModelFromJson(json);
}
