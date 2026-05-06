import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/presentation/state/pending_message.dart';

part 'chat_state.freezed.dart';

/// Терминальные сбои реалтайма (только типы; локализация в 11.10).
@freezed
abstract class ChatRealtimeSessionFailure with _$ChatRealtimeSessionFailure {
  const factory ChatRealtimeSessionFailure.authenticationLost() =
      _ChatRealtimeSessionFailureAuth;
  const factory ChatRealtimeSessionFailure.connectionLimitExceeded() =
      _ChatRealtimeSessionFailureConnLimit;
  const factory ChatRealtimeSessionFailure.forbidden() =
      _ChatRealtimeSessionFailureForbidden;
}

/// Состояние экрана чата (метаданные + история + пагинация + pending отправки).
///
/// **Провайдер:** [ChatController] с **`@Riverpod(keepAlive: true)`** — инстанс не
/// сбрасывается при уходе с экрана так же агрессивно, как у autoDispose; pending
/// и кэш списка переживают обращения только к **`.notifier`**. Экран **11.5**
/// по-прежнему должен **`watch`** провайдер для обновления UI.
@freezed
abstract class ChatState with _$ChatState {
  const factory ChatState({
    ConversationModel? conversation,
    @Default(<ConversationMessageModel>[])
    List<ConversationMessageModel> messages,
    @Default(true) bool isLoadingInitial,
    @Default(false) bool isLoadingOlder,
    @Default(false) bool hasMoreOlder,
    @Default(0) int olderOffset,
    @Default(<String, PendingMessage>{})
    Map<String, PendingMessage> pendingByClientId,
    /// Транзиентный сбой WS (сетевая ошибка и т.п.); не заменяет ленту сообщений.
    @Default(null) WsServiceFailure? realtimeServiceFailure,
    /// Терминальный сбой сессии (auth, лимит коннектов) — блок отправки до восстановления.
    @Default(null) ChatRealtimeSessionFailure? realtimeSessionFailure,
  }) = _ChatState;

  const ChatState._();

  factory ChatState.initial() => const ChatState();
}
