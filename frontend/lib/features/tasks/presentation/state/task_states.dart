import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';

part 'task_states.freezed.dart';

/// Мутация жизненного цикла задачи в полёте (12.8).
enum TaskLifecycleMutation {
  pause,
  cancel,
  resume,
}

/// Состояние списка задач проекта (Kanban / таблица — UI в 12.4).
@freezed
abstract class TaskListState with _$TaskListState {
  const factory TaskListState({
    @Default(<TaskListItemModel>[]) List<TaskListItemModel> items,
    @Default(0) int total,
    @Default(0) int offset,
    @Default(true) bool isLoadingInitial,
    @Default(false) bool isLoadingMore,
    @Default(false) bool hasMore,
    required TaskListFilter filter,
    @Default(false) bool realtimeMutationBlocked,
    RealtimeSessionFailure? realtimeSessionFailure,
    WsServiceFailure? realtimeServiceFailure,
    /// Ошибка пагинации списка (не переводит провайдер в AsyncError).
    Object? loadMoreError,
  }) = _TaskListState;

  const TaskListState._();

  factory TaskListState.initial() =>
      TaskListState(filter: TaskListFilter.defaults());
}

/// Состояние экрана деталей задачи (12.5).
@freezed
abstract class TaskDetailState with _$TaskDetailState {
  const factory TaskDetailState({
    TaskModel? task,
    @Default(<TaskMessageModel>[]) List<TaskMessageModel> messages,
    @Default(0) int messagesTotal,
    @Default(0) int messagesOffset,
    @Default(false) bool hasMoreMessages,
    String? messageTypeFilter,
    String? senderTypeFilter,
    @Default(true) bool isLoadingTask,
    @Default(false) bool isLoadingMessages,
    RealtimeSessionFailure? realtimeSessionFailure,
    @Default(false) bool realtimeMutationBlocked,
    WsServiceFailure? realtimeServiceFailure,
    @Default(false) bool taskDeleted,
    /// Ошибка догрузки сообщений (провайдер остаётся AsyncData).
    Object? messagesLoadMoreError,
    /// Текущая lifecycle-мутация (mutex pause/cancel/resume), только из контроллера (12.8).
    TaskLifecycleMutation? lifecycleMutationInFlight,
  }) = _TaskDetailState;

  const TaskDetailState._();

  factory TaskDetailState.initial() => const TaskDetailState();
}
