import 'package:frontend/features/tasks/domain/models.dart';

/// Поля списка из полной карточки задачи (без полей, отсутствующих в [TaskModel]).
TaskListItemModel taskModelToListItem(TaskModel m) {
  final item = TaskListItemModel(
    id: m.id,
    projectId: m.projectId,
    parentTaskId: m.parentTaskId,
    title: m.title,
    status: m.status,
    priority: m.priority,
    assignedAgent: m.assignedAgent,
    createdByType: m.createdByType,
    createdById: m.createdById,
    branchName: m.branchName,
    startedAt: m.startedAt,
    completedAt: m.completedAt,
    createdAt: m.createdAt,
    updatedAt: m.updatedAt,
  );

  assert(() {
    void checkNonNullMapped<T>(
      T? srcField,
      T itemField,
      String fieldName,
    ) {
      if (srcField != null && srcField != itemField) {
        throw StateError(
          'taskModelToListItem: поле $fieldName потеряно при маппинге',
        );
      }
    }

    checkNonNullMapped(m.parentTaskId, item.parentTaskId, 'parentTaskId');
    checkNonNullMapped(m.branchName, item.branchName, 'branchName');
    checkNonNullMapped(m.startedAt, item.startedAt, 'startedAt');
    checkNonNullMapped(m.completedAt, item.completedAt, 'completedAt');
    if (m.assignedAgent != null && item.assignedAgent != m.assignedAgent) {
      throw StateError('taskModelToListItem: assigned_agent искажён');
    }
    return true;
  }());

  return item;
}
