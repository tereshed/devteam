import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';

/// Локальная проверка соответствия задачи активным фильтрам списка (**без** [TaskListFilter.search] —
/// только серверный полнотекст).
bool taskMatchesCurrentFilter(TaskModel task, TaskListFilter filter) {
  return _matches(task.status, task.priority, task.assignedAgent?.id,
      task.createdByType, task.createdById, task.parentTaskId, task.branchName, filter);
}

/// Для строки Kanban / upsert после HTTP-мутации (те же правила, что [taskMatchesCurrentFilter],
/// но уже по [TaskListItemModel] без повторного маппинга полей вручную).
bool taskListItemMatchesCurrentFilter(
  TaskListItemModel item,
  TaskListFilter filter,
) {
  return _matches(item.status, item.priority, item.assignedAgent?.id,
      item.createdByType, item.createdById, item.parentTaskId, item.branchName, filter);
}

bool _matches(
  String status,
  String priority,
  String? assignedAgentId,
  String createdByType,
  String createdById,
  String? parentTaskId,
  String? branchName,
  TaskListFilter filter,
) {
  final st = filter.status;
  if (st != null && st.isNotEmpty && status != st) {
    return false;
  }

  final stList = filter.statuses;
  if (stList != null && stList.isNotEmpty && !stList.contains(status)) {
    return false;
  }

  final pr = filter.priority;
  if (pr != null && pr.isNotEmpty && priority != pr) {
    return false;
  }

  final agentId = filter.assignedAgentId;
  if (agentId != null && agentId.isNotEmpty && assignedAgentId != agentId) {
    return false;
  }

  final cbt = filter.createdByType;
  if (cbt != null && cbt.isNotEmpty && createdByType != cbt) {
    return false;
  }

  final cbid = filter.createdById;
  if (cbid != null && cbid.isNotEmpty && createdById != cbid) {
    return false;
  }

  final parentId = filter.parentTaskId;
  if (parentId != null && parentId.isNotEmpty && parentTaskId != parentId) {
    return false;
  }

  if (filter.rootOnly == true && parentTaskId != null && parentTaskId.isNotEmpty) {
    return false;
  }

  final branch = filter.branchName;
  if (branch != null && branch.isNotEmpty && branchName != branch) {
    return false;
  }

  return true;
}
