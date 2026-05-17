import 'package:freezed_annotation/freezed_annotation.dart';

part 'assistant_active_task_model.freezed.dart';
part 'assistant_active_task_model.g.dart';

/// Карточка активной задачи для Tasks-tab правой панели (Sprint 21 §4 backend).
///
/// Источник: `GET /assistant/active-tasks` (bootstrap) + `assistant.task_update`
/// WS-события (live апдейты). Тап ведёт на /projects/:project_id/tasks/:task_id.
@freezed
abstract class AssistantActiveTaskModel with _$AssistantActiveTaskModel {
  const factory AssistantActiveTaskModel({
    @JsonKey(name: 'task_id') required String taskId,
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'project_name') required String projectName,
    required String title,
    required String state,
    @JsonKey(name: 'updated_at') required DateTime updatedAt,
  }) = _AssistantActiveTaskModel;

  const AssistantActiveTaskModel._();

  factory AssistantActiveTaskModel.fromJson(Map<String, dynamic> json) =>
      _$AssistantActiveTaskModelFromJson(json);
}

@freezed
abstract class AssistantActiveTasksResponse
    with _$AssistantActiveTasksResponse {
  const factory AssistantActiveTasksResponse({
    @Default(<AssistantActiveTaskModel>[])
    List<AssistantActiveTaskModel> tasks,
  }) = _AssistantActiveTasksResponse;

  factory AssistantActiveTasksResponse.fromJson(Map<String, dynamic> json) =>
      _$AssistantActiveTasksResponseFromJson(json);
}
