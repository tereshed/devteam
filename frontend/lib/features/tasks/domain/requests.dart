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

  const TaskListResponse._();

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

  const TaskMessageListResponse._();

  factory TaskMessageListResponse.fromJson(Map<String, dynamic> json) =>
      _$TaskMessageListResponseFromJson(json);
}
