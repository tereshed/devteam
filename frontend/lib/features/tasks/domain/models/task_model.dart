import 'package:freezed_annotation/freezed_annotation.dart';

part 'task_model.freezed.dart';
part 'task_model.g.dart';

/// Роли агента (`AgentRole` на бэкенде).
const agentRoles = [
  'worker',
  'supervisor',
  'orchestrator',
  'planner',
  'developer',
  'reviewer',
  'tester',
  'devops',
];

/// Статусы задачи (`TaskStatus` на бэкенде).
const taskStatuses = [
  'pending',
  'planning',
  'in_progress',
  'review',
  'changes_requested',
  'testing',
  'completed',
  'failed',
  'cancelled',
  'paused',
];

/// Частые статусы для сравнений в UI вместо magic-string.
/// Значение [taskStatuses]: ожидает исполнения.
const kTaskStatusPending = 'pending';

/// Значение [taskStatuses]: в работе.
const kTaskStatusInProgress = 'in_progress';

/// Значение [taskStatuses]: успешно завершена.
const kTaskStatusCompleted = 'completed';

/// Значение [taskStatuses]: завершилась с ошибкой.
const kTaskStatusFailed = 'failed';

/// Значение [taskStatuses]: отменена пользователем.
const kTaskStatusCancelled = 'cancelled';

/// Приоритеты (`TaskPriority`).
const taskPriorities = ['critical', 'high', 'medium', 'low'];

/// Кто создал задачу (`CreatedByType`).
const createdByTypes = ['user', 'agent'];

/// Значение [createdByTypes] для пользователя.
const kCreatedByTypeUser = 'user';

/// Значение [createdByTypes] для агента.
const kCreatedByTypeAgent = 'agent';

/// Краткие данные об агенте для вложения в задачу (`AgentSummary` в API).
@freezed
abstract class AgentSummaryModel with _$AgentSummaryModel {
  const factory AgentSummaryModel({
    /// UUID агента
    required String id,

    /// Отображаемое имя
    required String name,

    /// Роль — строка из [agentRoles]
    required String role,
  }) = _AgentSummaryModel;

  const AgentSummaryModel._();

  factory AgentSummaryModel.fromJson(Map<String, dynamic> json) =>
      _$AgentSummaryModelFromJson(json);
}

/// Краткие данные о подзадаче (`TaskSummary` в API).
@freezed
abstract class TaskSummaryModel with _$TaskSummaryModel {
  const factory TaskSummaryModel({
    /// UUID подзадачи
    required String id,

    /// Краткое название подзадачи
    required String title,

    /// Статус — строка из [taskStatuses]
    required String status,

    /// Приоритет — строка из [taskPriorities]
    required String priority,
  }) = _TaskSummaryModel;

  const TaskSummaryModel._();

  factory TaskSummaryModel.fromJson(Map<String, dynamic> json) =>
      _$TaskSummaryModelFromJson(json);
}

/// Полная карточка задачи (`TaskResponse` в API).
@freezed
abstract class TaskModel with _$TaskModel {
  const factory TaskModel({
    /// UUID задачи
    required String id,

    /// UUID проекта
    @JsonKey(name: 'project_id')
    required String projectId,

    /// Родительская задача (если есть)
    @JsonKey(name: 'parent_task_id')
    String? parentTaskId,

    /// Краткое название задачи
    required String title,

    /// Подробное описание
    required String description,

    /// Статус — строка из [taskStatuses]
    required String status,

    /// Приоритет — строка из [taskPriorities]
    required String priority,

    /// Назначенный агент (если есть; ключ может отсутствовать в JSON)
    @JsonKey(name: 'assigned_agent')
    AgentSummaryModel? assignedAgent,

    /// Кто создал задачу — строка из [createdByTypes]
    @JsonKey(name: 'created_by_type')
    required String createdByType,

    /// UUID создателя; семантика задаётся [createdByType] (user vs agent)
    @JsonKey(name: 'created_by_id')
    required String createdById,

    /// Контекст задачи (JSONB)
    @Default(<String, dynamic>{})
    Map<String, dynamic> context,

    /// Текст результата задачи. Для UI: проверяй ещё непустоту (`null` ≠ `""`).
    String? result,

    /// Артефакты (JSONB)
    @Default(<String, dynamic>{})
    Map<String, dynamic> artifacts,

    /// Имя Git-ветки для этой задачи
    @JsonKey(name: 'branch_name')
    String? branchName,

    /// Доменное сообщение об ошибке задачи (например, при status=`failed`).
    /// НЕ путать с телом HTTP-ошибки: разбор 4xx/5xx — в `dio_api_error.dart`.
    @JsonKey(name: 'error_message')
    String? errorMessage,

    /// Подзадачи; с бэкенда обычно в порядке `created_at ASC`
    @JsonKey(name: 'sub_tasks')
    @Default(<TaskSummaryModel>[])
    List<TaskSummaryModel> subTasks,

    /// Зарезервировано: бэкенд пока не выставляет (`omitempty` → ключ обычно отсутствует).
    /// Оставлено на случай заполнения в `task_handler` в будущем.
    @JsonKey(name: 'message_count')
    int? messageCount,

    /// Время начала выполнения
    @JsonKey(name: 'started_at')
    DateTime? startedAt,

    /// Время завершения
    @JsonKey(name: 'completed_at')
    DateTime? completedAt,

    /// Дата создания
    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    /// Дата последнего обновления
    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _TaskModel;

  const TaskModel._();

  factory TaskModel.fromJson(Map<String, dynamic> json) =>
      _$TaskModelFromJson(json);
}
