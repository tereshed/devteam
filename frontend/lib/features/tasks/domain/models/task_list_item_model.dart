import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';

part 'task_list_item_model.freezed.dart';
part 'task_list_item_model.g.dart';

/// Строка списка задач проекта (`TaskListItem` в API; без JSONB и подзадач).
@freezed
abstract class TaskListItemModel with _$TaskListItemModel {
  const factory TaskListItemModel({
    /// UUID задачи
    required String id,

    /// UUID проекта
    @JsonKey(name: 'project_id')
    required String projectId,

    /// Родительская задача (если есть)
    @JsonKey(name: 'parent_task_id')
    String? parentTaskId,

    /// Краткое название
    required String title,

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

    /// UUID создателя; семантика — [createdByType]
    @JsonKey(name: 'created_by_id')
    required String createdById,

    /// Имя Git-ветки для этой задачи
    @JsonKey(name: 'branch_name')
    String? branchName,

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
  }) = _TaskListItemModel;

  const TaskListItemModel._();

  factory TaskListItemModel.fromJson(Map<String, dynamic> json) =>
      _$TaskListItemModelFromJson(json);
}
