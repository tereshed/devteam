import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/tasks/domain/models.dart';

part 'requests.freezed.dart';
part 'requests.g.dart';

/// Ответ `GET /projects/:id/tasks`: страница списка задач (без `has_next`).
@freezed
abstract class TaskListResponse with _$TaskListResponse {
  const factory TaskListResponse({
    /// Элементы текущей страницы
    @Default(<TaskListItemModel>[]) List<TaskListItemModel> tasks,

    /// Всего записей по запросу
    @Default(0) int total,

    /// Размер страницы
    @Default(0) int limit,

    /// Смещение
    @Default(0) int offset,
  }) = _TaskListResponse;

  factory TaskListResponse.fromJson(Map<String, dynamic> json) =>
      _$TaskListResponseFromJson(json);
}

/// Ответ `GET /tasks/:id/messages`: страница сообщений задачи (без `has_next`).
@freezed
abstract class TaskMessageListResponse with _$TaskMessageListResponse {
  const factory TaskMessageListResponse({
    /// Сообщения текущей страницы
    @Default(<TaskMessageModel>[]) List<TaskMessageModel> messages,

    /// Всего записей по запросу
    @Default(0) int total,

    /// Размер страницы
    @Default(0) int limit,

    /// Смещение
    @Default(0) int offset,
  }) = _TaskMessageListResponse;

  factory TaskMessageListResponse.fromJson(Map<String, dynamic> json) =>
      _$TaskMessageListResponseFromJson(json);
}

// ---------------------------------------------------------------------------
// Query / filters (Sprint 12.2)
// ---------------------------------------------------------------------------

/// Направление сортировки списка задач; на wire только `ASC` / `DESC` (см. `sanitizeOrderDir` на бэкенде).
enum TaskOrderDir {
  asc,
  desc;

  /// Значение query `order_dir` для API.
  String toWire() => switch (this) {
        TaskOrderDir.asc => 'ASC',
        TaskOrderDir.desc => 'DESC',
      };
}

/// Фильтры `GET /projects/:id/tasks` (без пагинации: [limit]/[offset] — аргументы [TaskRepository.listTasks]).
@freezed
abstract class TaskListFilter with _$TaskListFilter {
  const factory TaskListFilter({
    String? status,
    List<String>? statuses,
    String? priority,
    String? assignedAgentId,
    String? createdByType,
    String? createdById,
    String? parentTaskId,
    bool? rootOnly,
    String? branchName,
    String? search,
    String? orderBy,
    TaskOrderDir? orderDir,
  }) = _TaskListFilter;

  const TaskListFilter._();

  /// Параметры query для Dio (`statuses` — список для `ListFormat.multi`).
  /// Имена ключей — контракт HTTP (`ListTasksRequest` на бэкенде), не `@JsonKey`.
  Map<String, dynamic> toQueryParameters() {
    final st = status;
    final stList = statuses;
    final pr = priority;
    final agentId = assignedAgentId;
    final cbt = createdByType;
    final cbid = createdById;
    final parentId = parentTaskId;
    final branch = branchName;
    final q = search;
    final ob = orderBy;
    final od = orderDir;

    return <String, dynamic>{
      if (st != null && st.isNotEmpty) 'status': st,
      if (stList != null && stList.isNotEmpty) 'statuses': stList,
      if (pr != null && pr.isNotEmpty) 'priority': pr,
      if (agentId != null && agentId.isNotEmpty) 'assigned_agent_id': agentId,
      if (cbt != null && cbt.isNotEmpty) 'created_by_type': cbt,
      if (cbid != null && cbid.isNotEmpty) 'created_by_id': cbid,
      if (parentId != null && parentId.isNotEmpty) 'parent_task_id': parentId,
      if (rootOnly == true) 'root_only': true,
      if (branch != null && branch.isNotEmpty) 'branch_name': branch,
      if (q != null && q.isNotEmpty) 'search': q,
      if (ob != null && ob.isNotEmpty) 'order_by': ob,
      if (od != null) 'order_dir': od.toWire(),
    };
  }
}

/// Тело `POST /projects/:id/tasks` (см. `CreateTaskRequest` в Go).
@freezed
abstract class CreateTaskRequest with _$CreateTaskRequest {
  @JsonSerializable(includeIfNull: false)
  const factory CreateTaskRequest({
    required String title,
    @Default('') String description,
    @Default('') String priority,
    @JsonKey(name: 'parent_task_id') String? parentTaskId,
    @JsonKey(name: 'assigned_agent_id') String? assignedAgentId,
    @Default(<String, dynamic>{}) Map<String, dynamic> context,
  }) = _CreateTaskRequest;

  factory CreateTaskRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateTaskRequestFromJson(json);
}

/// Тело `PUT /tasks/:id` (частичное обновление).
@freezed
abstract class UpdateTaskRequest with _$UpdateTaskRequest {
  @JsonSerializable(includeIfNull: false)
  const factory UpdateTaskRequest({
    String? title,
    String? description,
    String? priority,

    /// Обычно изменение статуса выполняется через [TaskRepository.pauseTask],
    /// [TaskRepository.cancelTask], [TaskRepository.resumeTask]. Поле [status] использовать только
    /// когда фича явно требует прямого изменения (например ручная пометка `failed` админом);
    /// иначе легко обойти ожидаемый поток и проверки переходов на бэкенде.
    String? status,
    @JsonKey(name: 'assigned_agent_id') String? assignedAgentId,
    @JsonKey(name: 'clear_assigned_agent')
    @Default(false)
    bool clearAssignedAgent,
    @JsonKey(name: 'branch_name') String? branchName,
  }) = _UpdateTaskRequest;

  factory UpdateTaskRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateTaskRequestFromJson(json);
}

/// Тело `POST /tasks/:id/correct`.
@freezed
abstract class CorrectTaskRequest with _$CorrectTaskRequest {
  const factory CorrectTaskRequest({
    required String text,
  }) = _CorrectTaskRequest;

  factory CorrectTaskRequest.fromJson(Map<String, dynamic> json) =>
      _$CorrectTaskRequestFromJson(json);
}

/// Тело `POST /tasks/:id/messages`.
@freezed
abstract class CreateTaskMessageRequest with _$CreateTaskMessageRequest {
  const factory CreateTaskMessageRequest({
    required String content,
    @JsonKey(name: 'message_type') required String messageType,
    @Default(<String, dynamic>{}) Map<String, dynamic> metadata,
  }) = _CreateTaskMessageRequest;

  factory CreateTaskMessageRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateTaskMessageRequestFromJson(json);
}
