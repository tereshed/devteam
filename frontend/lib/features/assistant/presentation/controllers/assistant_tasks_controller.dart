import 'dart:async';

import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_active_task_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_tasks_controller.g.dart';

/// Состояние Tasks-tab правой панели (Sprint 21 §11 frontend).
///
/// Источники:
/// - **bootstrap snapshot**: `GET /assistant/active-tasks` (после login и
///   при ручном refresh);
/// - **live updates**: WS-событие `assistant.task_update` (user-scoped,
///   эмитится HubBridge'ом параллельно с project-scoped `task_status`).
///
/// Хранение: упорядоченный по `updatedAt` DESC список карточек.
class AssistantTasksState {
  const AssistantTasksState({
    this.loading = false,
    this.tasks = const <AssistantActiveTaskModel>[],
    this.error,
  });

  /// Идёт REST bootstrap (или ручной refresh).
  final bool loading;

  /// Активные задачи всех проектов пользователя, упорядочены по `updatedAt`
  /// DESC. Терминальные state'ы (`completed|failed|cancelled`) остаются в
  /// списке до явного [reset] / refresh, чтобы анимация перехода «active →
  /// done» была заметной (план §11).
  final List<AssistantActiveTaskModel> tasks;

  final AssistantRepositoryException? error;

  AssistantTasksState copyWith({
    bool? loading,
    List<AssistantActiveTaskModel>? tasks,
    Object? error = _sentinel,
  }) {
    return AssistantTasksState(
      loading: loading ?? this.loading,
      tasks: tasks ?? this.tasks,
      error: identical(error, _sentinel)
          ? this.error
          : error as AssistantRepositoryException?,
    );
  }

  static const Object _sentinel = Object();
}

/// Контроллер вкладки **«Tasks»** правой панели ассистента (Sprint 21 §11).
///
/// **Все** проекты пользователя — нет project-фильтра, в отличие от
/// `ChatController`/`TaskListController` (см. assistant-sidebar plan §7).
///
/// Идемпотентность: события дедуплицируются по `taskId`, всегда побеждает
/// самое свежее `updatedAt` (защита от out-of-order при reconnect).
@Riverpod(keepAlive: true)
class AssistantTasksController extends _$AssistantTasksController {
  StreamSubscription<WsClientEvent>? _wsSubscription;

  @override
  AssistantTasksState build() {
    final ws = ref.read(webSocketServiceProvider);
    _wsSubscription = ws.events.listen(_onWsClientEvent);
    ref.onDispose(() {
      unawaited(_wsSubscription?.cancel());
      _wsSubscription = null;
    });
    return const AssistantTasksState();
  }

  /// REST bootstrap. Вызывает UI при первом открытии вкладки и pull-to-refresh.
  Future<void> refresh() async {
    if (state.loading) return;
    state = state.copyWith(loading: true, error: null);
    final repo = ref.read(assistantRepositoryProvider);
    try {
      final resp = await repo.getActiveTasks();
      final sorted = [...resp.tasks]
        ..sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
      state = state.copyWith(loading: false, tasks: sorted);
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(loading: false, error: e);
    }
  }

  /// Сбрасывает локальное состояние (например, при logout).
  void reset() {
    state = const AssistantTasksState();
  }

  void clearError() {
    if (state.error == null) return;
    state = state.copyWith(error: null);
  }

  void _onWsClientEvent(WsClientEvent ev) {
    if (ev is! WsClientEventServer) return;
    final assistantTaskUpdate = ev.event.maybeMap(
      assistantTaskUpdate: (e) => e.value,
      orElse: () => null,
    );
    if (assistantTaskUpdate == null) return;
    _applyTaskUpdate(assistantTaskUpdate);
  }

  void _applyTaskUpdate(WsAssistantTaskUpdateEvent update) {
    final existing = state.tasks.firstWhere(
      (t) => t.taskId == update.taskId,
      orElse: () => _missing,
    );
    if (!identical(existing, _missing) &&
        !update.updatedAt.isAfter(existing.updatedAt)) {
      // Out-of-order guard.
      return;
    }
    // Строим новую карточку. Бэкенд-обновление НЕ несёт project_name —
    // оно идёт только из REST snapshot. Если задача неизвестна (карточка
    // ещё не пришла из REST), мы можем создать минимальную row с пустым
    // project_name; UI покажет её как «(unknown project)» — это лучше, чем
    // пропустить апдейт. REST refresh затем заполнит правильно.
    final updated = AssistantActiveTaskModel(
      taskId: update.taskId,
      projectId: update.projectId,
      projectName: identical(existing, _missing) ? '' : existing.projectName,
      title: update.title ?? (identical(existing, _missing) ? '' : existing.title),
      state: update.state,
      updatedAt: update.updatedAt,
    );
    final next = identical(existing, _missing)
        ? [updated, ...state.tasks]
        : [
            for (final t in state.tasks)
              if (t.taskId == update.taskId) updated else t,
          ];
    next.sort((a, b) => b.updatedAt.compareTo(a.updatedAt));
    state = state.copyWith(tasks: next);
  }

  // Sentinel для «не найдено» без двойного прохода списка.
  static final AssistantActiveTaskModel _missing = AssistantActiveTaskModel(
    taskId: '',
    projectId: '',
    projectName: '',
    title: '',
    state: '',
    updatedAt: DateTime.fromMillisecondsSinceEpoch(0),
  );
}
