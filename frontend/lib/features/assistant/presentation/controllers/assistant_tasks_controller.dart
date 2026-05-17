import 'dart:async';

import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_tasks_controller.g.dart';

/// Контроллер вкладки **«Tasks»** правой панели ассистента (Sprint 21 §7/§11).
///
/// Подписан на user-scoped событие `assistant.task_update`, эмитимое
/// HubBridge'м параллельно с project-scoped `task_status` при смене state
/// (см. `backend/internal/ws/hubbridge.go fanOutAssistantTaskUpdate`).
///
/// Контракт:
/// - **Все** проекты пользователя — нет project-фильтра, в отличие от
///   `ChatController`/`TaskListController` (см. assistant-sidebar plan §7).
/// - Не делает REST-запросов: первичный bootstrap (initial snapshot) — задача
///   §11 (Tasks-panel UI). Здесь только реактивный поток событий, который UI
///   мержит со своим snapshot'ом.
/// - Идемпотентность: события дедуплицируются по `taskId`, всегда побеждает
///   самое свежее `updatedAt` (защита от out-of-order при reconnect).
///
/// Терминальные задачи (`done|cancelled|failed`) **остаются** в карте на
/// последний снимок — UI сам решает, фильтровать ли по `state=active`.
/// Это нужно, чтобы анимация перехода `active → done` была визуально
/// заметной (карточка не пропадает мгновенно).
@Riverpod(keepAlive: true)
class AssistantTasksController extends _$AssistantTasksController {
  StreamSubscription<WsClientEvent>? _wsSubscription;

  @override
  Map<String, WsAssistantTaskUpdateEvent> build() {
    final ws = ref.read(webSocketServiceProvider);
    _wsSubscription = ws.events.listen(_onWsClientEvent);
    ref.onDispose(() {
      unawaited(_wsSubscription?.cancel());
      _wsSubscription = null;
    });
    return const <String, WsAssistantTaskUpdateEvent>{};
  }

  /// Сбрасывает локальное состояние (например, при logout). UI-bootstrap
  /// перезаливает snapshot через REST `/api/v1/assistant/active-tasks`.
  void reset() {
    state = const <String, WsAssistantTaskUpdateEvent>{};
  }

  void _onWsClientEvent(WsClientEvent ev) {
    if (ev is! WsClientEventServer) {
      return;
    }
    final assistantTaskUpdate = ev.event.maybeMap(
      assistantTaskUpdate: (e) => e.value,
      orElse: () => null,
    );
    if (assistantTaskUpdate == null) {
      return;
    }
    _applyTaskUpdate(assistantTaskUpdate);
  }

  void _applyTaskUpdate(WsAssistantTaskUpdateEvent update) {
    final existing = state[update.taskId];
    // Out-of-order guard: если старое событие догнало нас позже свежего —
    // игнорируем (defensive: backend гарантирует at-least-once, но при
    // reconnect пачка может прилететь не строго в порядке updatedAt).
    if (existing != null && !update.updatedAt.isAfter(existing.updatedAt)) {
      return;
    }
    state = {
      ...state,
      update.taskId: update,
    };
  }
}
