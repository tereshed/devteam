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
    @JsonKey(name: 'llm_provider_id') String? llmProviderID,
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
const List<String> kSupportedCodeBackends = [
  'claude-code',
  'claude-code-via-proxy',
  'aider',
  'custom',
];

/// Допустимые значения permissions.defaultMode (Claude Code CLI).
const List<String> kSupportedPermissionModes = [
  'default',
  'acceptEdits',
  'plan',
  'bypassPermissions',
];

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
