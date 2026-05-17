import 'dart:async';

import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_chat_controller.g.dart';

/// Состояние ассистент-чата правой панели (Sprint 21 §7/§10).
///
/// Это **минимальная** stage-7 версия: только реактивный «последний снимок»
/// каждого канала событий. Полноценный list-of-bubbles state, slot для
/// pending confirm-диалога, busy-флаг сессии и связка с REST-историей —
/// в стадиях §10/§12.
///
/// Сейчас достаточно, чтобы UI мог:
/// - смотреть `currentSessionId` и переключаться между sessions;
/// - получать last [WsAssistantMessageEvent] для триггера scroll-to-bottom;
/// - получать last [WsAssistantConfirmRequestEvent] для триггера диалога;
/// - получать last [WsAssistantNavigateEvent] для `go_router.go()`.
class AssistantChatState {
  const AssistantChatState({
    this.currentSessionId,
    this.lastSessionUpdated,
    this.lastMessage,
    this.lastToolCall,
    this.lastToolResult,
    this.pendingConfirm,
    this.lastNavigate,
  });

  /// SessionID, на который UI «смотрит» сейчас. События с другим
  /// `session_id` пропускаются на уровне контроллера, чтобы фон не дёргал
  /// активный диалог.
  final String? currentSessionId;
  final WsAssistantSessionUpdatedEvent? lastSessionUpdated;
  final WsAssistantMessageEvent? lastMessage;
  final WsAssistantToolCallEvent? lastToolCall;
  final WsAssistantToolResultEvent? lastToolResult;

  /// Pending confirm-request, ожидающий Approve/Deny от пользователя.
  /// Очищается после успешного `POST /assistant/sessions/:id/confirm`.
  final WsAssistantConfirmRequestEvent? pendingConfirm;

  final WsAssistantNavigateEvent? lastNavigate;

  AssistantChatState copyWith({
    String? currentSessionId,
    WsAssistantSessionUpdatedEvent? lastSessionUpdated,
    WsAssistantMessageEvent? lastMessage,
    WsAssistantToolCallEvent? lastToolCall,
    WsAssistantToolResultEvent? lastToolResult,
    Object? pendingConfirm = _sentinel,
    WsAssistantNavigateEvent? lastNavigate,
  }) {
    return AssistantChatState(
      currentSessionId: currentSessionId ?? this.currentSessionId,
      lastSessionUpdated: lastSessionUpdated ?? this.lastSessionUpdated,
      lastMessage: lastMessage ?? this.lastMessage,
      lastToolCall: lastToolCall ?? this.lastToolCall,
      lastToolResult: lastToolResult ?? this.lastToolResult,
      pendingConfirm: identical(pendingConfirm, _sentinel)
          ? this.pendingConfirm
          : pendingConfirm as WsAssistantConfirmRequestEvent?,
      lastNavigate: lastNavigate ?? this.lastNavigate,
    );
  }

  static const Object _sentinel = Object();
}

/// Реактивный контроллер ассистент-чата. Минимальный stage-7 контракт —
/// см. doc-комментарий к [AssistantChatState].
///
/// Stage-7 контракт фильтрации:
/// - `currentSessionId == null` → принимаем все события (UI ещё не выбрал
///   сессию; обычно сразу после login).
/// - `currentSessionId != null` → пропускаем события чужих сессий.
///
/// `assistant.task_update` — НЕ обрабатывается здесь, для него отдельный
/// [AssistantTasksController].
@Riverpod(keepAlive: true)
class AssistantChatController extends _$AssistantChatController {
  StreamSubscription<WsClientEvent>? _wsSubscription;

  @override
  AssistantChatState build() {
    final ws = ref.read(webSocketServiceProvider);
    _wsSubscription = ws.events.listen(_onWsClientEvent);
    ref.onDispose(() {
      unawaited(_wsSubscription?.cancel());
      _wsSubscription = null;
    });
    return const AssistantChatState();
  }

  /// Переключение «активной» сессии в UI. Очищает pendingConfirm чужой
  /// сессии: иначе при переключении остался бы зависший диалог.
  void setCurrentSession(String? sessionId) {
    state = state.copyWith(
      currentSessionId: sessionId,
      pendingConfirm: null,
    );
  }

  /// Снимает pendingConfirm после успешного `POST /confirm`. UI должен
  /// вызывать после получения 2xx-ответа от backend.
  void clearPendingConfirm() {
    state = state.copyWith(pendingConfirm: null);
  }

  void _onWsClientEvent(WsClientEvent ev) {
    if (ev is! WsClientEventServer) {
      return;
    }
    ev.event.maybeMap(
      assistantSessionUpdated: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(lastSessionUpdated: e.value);
        return null;
      },
      assistantMessage: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(lastMessage: e.value);
        return null;
      },
      assistantToolCall: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(lastToolCall: e.value);
        return null;
      },
      assistantToolResult: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(lastToolResult: e.value);
        return null;
      },
      assistantConfirmRequest: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(pendingConfirm: e.value);
        return null;
      },
      assistantNavigate: (e) {
        // navigate — user-scoped, без session_id. Применяем всегда.
        state = state.copyWith(lastNavigate: e.value);
        return null;
      },
      orElse: () => null,
    );
  }

  bool _matchesCurrent(String sessionId) {
    final current = state.currentSessionId;
    return current == null || current == sessionId;
  }
}
