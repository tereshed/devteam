import 'package:frontend/features/tasks/domain/models/task_list_item_model.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';

/// UUID проекта для фикстур задач (совпадает с тестами shell / 12.3).
const String kTaskFixtureProjectId = '550e8400-e29b-41d4-a716-446655440000';

/// UUID пользователя для полей created_by_id.
const String kTaskFixtureUserId = '33333333-3333-3333-3333-333333333333';

TaskListItemModel makeTaskListItemFixture({
  required String id,
  String title = 'Fixture task',
  String status = 'pending',
  String priority = 'medium',
  DateTime? createdAt,
  DateTime? updatedAt,
  AgentSummaryModel? assignedAgent,
}) {
  final c = createdAt ?? DateTime.utc(2026, 1, 1);
  final u = updatedAt ?? DateTime.utc(2026, 1, 2);
  return TaskListItemModel(
    id: id,
    projectId: kTaskFixtureProjectId,
    title: title,
    status: status,
    priority: priority,
    assignedAgent: assignedAgent,
    createdByType: 'user',
    createdById: kTaskFixtureUserId,
    createdAt: c,
    updatedAt: u,
  );
}

TaskListState makeTaskListStateFixture({
  List<TaskListItemModel> items = const [],
  int total = 0,
  int offset = 0,
  bool isLoadingInitial = false,
  bool isLoadingMore = false,
  bool hasMore = false,
  TaskListFilter? filter,
  Object? loadMoreError,
}) {
  return TaskListState(
    filter: filter ?? TaskListFilter.defaults(),
    items: items,
    total: total,
    offset: offset,
    isLoadingInitial: isLoadingInitial,
    isLoadingMore: isLoadingMore,
    hasMore: hasMore,
    loadMoreError: loadMoreError,
  );
}
