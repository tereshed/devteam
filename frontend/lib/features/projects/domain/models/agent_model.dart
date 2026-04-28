import 'package:freezed_annotation/freezed_annotation.dart';

part 'agent_model.freezed.dart';
part 'agent_model.g.dart';

@freezed
abstract class AgentModel with _$AgentModel {
  const factory AgentModel({
    /// UUID агента
    required String id,

    /// Название агента
    required String name,

    /// Роль агента в pipeline: 'planner', 'developer', 'reviewer', 'tester', 'orchestrator', 'worker', 'supervisor', 'devops'
    required String role,

    /// Имя модели LLM (например: 'claude-opus-4-7')
    /// Nullable: если не установлена, используется по умолчанию
    String? model,

    /// Имя промпта (относится к отдельной сущности Prompt в backend)
    /// Nullable: может быть не установлен
    @JsonKey(name: 'prompt_name')
    String? promptName,

    /// Бэкенд для выполнения (claude-code | aider | custom)
    /// Nullable: не все агенты выполняют код в sandbox
    @JsonKey(name: 'code_backend')
    String? codeBackend,

    /// Активен ли агент
    @JsonKey(name: 'is_active')
    required bool isActive,
  }) = _AgentModel;

  const AgentModel._();

  factory AgentModel.fromJson(Map<String, dynamic> json) =>
      _$AgentModelFromJson(json);
}

/// Роли агентов в pipeline
const agentRoles = [
  'orchestrator',  // Главный оркестратор
  'planner',       // Планировщик задач
  'developer',     // Разработчик (может использовать sandbox)
  'reviewer',      // Ревьюер кода
  'tester',        // Тестер
  'worker',        // Общая рабочая роль
  'supervisor',    // Надзиратель
  'devops',        // DevOps специалист
];

/// Бэкенды для sandbox
const codeBackends = [
  'claude-code',   // Claude Code CLI
  'aider',         // Aider
  'custom',        // Пользовательский бэкенд
];
