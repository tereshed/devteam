import 'package:freezed_annotation/freezed_annotation.dart';

part 'agent_config_model.freezed.dart';
part 'agent_config_model.g.dart';

@freezed
abstract class AgentConfigModel with _$AgentConfigModel {
  const factory AgentConfigModel({
    required String id,
    required String name,
    required String role,
    @JsonKey(name: 'execution_kind') required String executionKind,
    @JsonKey(name: 'provider_kind') String? providerKind,
    String? model,
    double? temperature,
    @JsonKey(name: 'internal_mcp_enabled') required bool internalMcpEnabled,
    @JsonKey(name: 'is_active') required bool isActive,
    @JsonKey(name: 'system_prompt') String? systemPrompt,
    @JsonKey(name: 'code_backend') String? codeBackend,
    @JsonKey(name: 'team_id') String? teamId,
    @JsonKey(name: 'user_id') String? userId,
    @JsonKey(name: 'created_at') String? createdAt,
    @JsonKey(name: 'updated_at') String? updatedAt,
  }) = _AgentConfigModel;

  factory AgentConfigModel.fromJson(Map<String, dynamic> json) =>
      _$AgentConfigModelFromJson(json);
}

@freezed
abstract class AgentSkillModel with _$AgentSkillModel {
  const factory AgentSkillModel({
    required String id,
    @JsonKey(name: 'agent_id') required String agentId,
    @JsonKey(name: 'skill_name') required String skillName,
    @JsonKey(name: 'skill_source') required String skillSource,
    @JsonKey(name: 'config_json') Map<String, dynamic>? configJson,
    @JsonKey(name: 'is_active') required bool isActive,
  }) = _AgentSkillModel;

  factory AgentSkillModel.fromJson(Map<String, dynamic> json) =>
      _$AgentSkillModelFromJson(json);
}

@freezed
abstract class SecretRefModel with _$SecretRefModel {
  const factory SecretRefModel({
    required String id,
    @JsonKey(name: 'key_name') required String keyName,
    @JsonKey(name: 'inject_as_env') @Default(false) bool injectAsEnv,
    @Default('') String description,
    @JsonKey(name: 'created_at') String? createdAt,
    @JsonKey(name: 'updated_at') String? updatedAt,
  }) = _SecretRefModel;

  factory SecretRefModel.fromJson(Map<String, dynamic> json) =>
      _$SecretRefModelFromJson(json);
}

@freezed
abstract class AgentRolePromptModel with _$AgentRolePromptModel {
  const factory AgentRolePromptModel({
    required String id,
    required String role,
    required String content,
    String? description,
    @JsonKey(name: 'updated_at') String? updatedAt,
    @JsonKey(name: 'updated_by') String? updatedBy,
  }) = _AgentRolePromptModel;

  factory AgentRolePromptModel.fromJson(Map<String, dynamic> json) =>
      _$AgentRolePromptModelFromJson(json);
}

const kAgentRoles = [
  'assistant',
  'orchestrator',
  'router',
  'planner',
  'developer',
  'reviewer',
  'tester',
  'decomposer',
  'merger',
];

const kAutoCreatedRoles = ['assistant', 'orchestrator', 'router'];

const kExecutionKinds = ['llm', 'sandbox'];
