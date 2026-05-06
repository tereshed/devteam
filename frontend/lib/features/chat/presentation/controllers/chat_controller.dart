import 'dart:async';

import 'package:dio/dio.dart' show CancelToken;
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/domain/linked_task_snapshots.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/chat/presentation/state/chat_state.dart';
import 'package:frontend/features/chat/presentation/state/pending_message.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'chat_controller.g.dart';

/// Результат [ChatController.send] / [ChatController.retrySend] для UI (11.9).
enum ChatSendOutcome {
  /// Пустой ввод или нет pending для retry — репозиторий не вызывали.
  ignored,
  /// [ChatState.realtimeSessionFailure] != null — HTTP не выполняли.
  blockedByRealtimeSession,
  /// Успех, постановка в transient pending или отмена отправки.
  completed,
}

/// Локализованный заголовок ошибки чата (SnackBar / диалоги).
String chatErrorTitle(AppLocalizations l10n, Object error) {
  return switch (error) {
    ConversationNotFoundException _ => l10n.chatErrorConversationNotFound,
    ConversationForbiddenException _ => l10n.errorForbidden,
    ConversationRateLimitedException _ => l10n.chatErrorRateLimited,
    UnauthorizedException _ => l10n.errorUnauthorized,
    ConversationCancelledException _ => l10n.errorRequestCancelled,
    final ConversationApiException e => _chatApiErrorTitle(l10n, e),
    _ => l10n.chatErrorGeneric,
  };
}

String _chatApiErrorTitle(AppLocalizations l10n, ConversationApiException e) {
  if (e.isNetworkTransportError) {
    return l10n.errorNetwork;
  }
  if ((e.statusCode ?? 0) >= 500) {
    return l10n.errorServer;
  }
  return l10n.chatErrorGeneric;
}

/// Короткий безопасный хвост детали ошибки (если есть).
String? chatErrorDetail(Object error) {
  if (error is ConversationCancelledException) {
    return null;
  }
  if (error is! ConversationApiException) {
    return null;
  }
  final m = error.message;
  if (m.isEmpty) {
    return null;
  }
  if (error.isNetworkTransportError) {
    return null;
  }
  const maxLen = 200;
  if (m.length <= maxLen) {
    return m;
  }
  var head = m.substring(0, maxLen);
  head = head.replaceAll(RegExp(r'(?:\.+|\u2026)+\s*$'), '');
  if (head.isEmpty) {
    head = m.substring(0, maxLen);
  }
  return '$head…';
}

/// Слияние сообщений по [ConversationMessageModel.id], затем сортировка
/// `created_at` ASC, tie-breaker `id` ASC (как на бэкенде).
List<ConversationMessageModel> _mergeMessagesCanonical(
  List<ConversationMessageModel> current,
  List<ConversationMessageModel> incoming,
) {
  final byId = <String, ConversationMessageModel>{};
  for (final m in current) {
    byId[m.id] = m;
  }
  for (final m in incoming) {
    byId[m.id] = m;
  }
  final out = byId.values.toList()
    ..sort((a, b) {
      final c = a.createdAt.compareTo(b.createdAt);
      if (c != 0) {
        return c;
      }
      return a.id.compareTo(b.id);
    });
  return out;
}

bool _hasMoreAfterPage({
  required MessageListResponse response,
  required Set<String> idsBeforeMerge,
}) {
  if (response.messages.isEmpty && response.hasNext) {
    return false;
  }
  var newFromPage = 0;
  for (final m in response.messages) {
    if (!idsBeforeMerge.contains(m.id)) {
      newFromPage++;
    }
  }
  return response.hasNext && newFromPage > 0;
}

bool _isTransientSendFailure(Object error) {
  if (error is ConversationApiException) {
    if (error.isNetworkTransportError) {
      return true;
    }
    final code = error.statusCode ?? 0;
    return code >= 500;
  }
  return false;
}

/// Контроллер чата: метаданные, история с offset-пагинацией, отправка с
/// идемпотентным ретраем, шов для реалтайма ([applyIncomingMessage] — 11.9).
///
/// **keepAlive:** `true`, чтобы при обращении только к `.notifier` (без `watch`
/// в UI) не срабатывал autoDispose до завершения фоновой `_loadInitial`.
///
/// **refresh:** отменяет только запросы истории ([_historyCancelToken]);
/// inflight [ConversationRepository.sendMessage] общим токеном не трогаются.
@Riverpod(keepAlive: true)
class ChatController extends _$ChatController {
  static const int _initialPageLimit = 20;
  static const int _olderPageLimit = 20;

  CancelToken? _historyCancelToken;
  int _sessionEpoch = 0;

  /// Inflight [loadOlder]; после [refresh] старый [Future] живёт до [whenComplete]
  /// — повторный [loadOlder] в этом окне получит тот же [Future] (уже отменённый
  /// запрос истории), это безопасно.
  Future<void>? _olderInFlight;

  StreamSubscription<WsClientEvent>? _wsSubscription;

  /// Антишторм [refresh] по `needsRestRefetch` (stream_overflow).
  bool _wsOverflowRefreshInFlight = false;

  @override
  FutureOr<ChatState> build({
    required String projectId,
    required String conversationId,
  }) {
    if (projectId.isEmpty || conversationId.isEmpty) {
      throw ArgumentError.value(
        projectId.isEmpty ? projectId : conversationId,
        'projectId/conversationId',
        'must be non-empty UUID strings',
      );
    }

    _historyCancelToken ??= CancelToken();

    if (_wsSubscription == null) {
      ref.onDispose(() {
        _historyCancelToken?.cancel();
        unawaited(_wsSubscription?.cancel());
        _wsSubscription = null;
      });
      final ws = ref.read(webSocketServiceProvider);
      ws.connect(projectId);
      _wsSubscription = ws.events.listen(_onWsClientEvent);
      Future.microtask(() {
        unawaited(_loadInitial());
      });
    }

    return ChatState.initial();
  }

  void _clearRealtimeTransientFailure() {
    final v = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (v == null || v.realtimeServiceFailure == null) {
      return;
    }
    _patchState((s) => s.copyWith(realtimeServiceFailure: null));
  }

  void _patchRealtimeSessionFailure(ChatRealtimeSessionFailure failure) {
    _patchState(
      (s) => s.copyWith(
        realtimeSessionFailure: failure,
        realtimeServiceFailure: null,
      ),
    );
  }

  void _applyTerminalServiceFailure(WsServiceFailure f) {
    f.when(
      transient: (_) {},
      authExpired: () => _patchRealtimeSessionFailure(
        const ChatRealtimeSessionFailure.authenticationLost(),
      ),
      policyForbidden: () => _patchRealtimeSessionFailure(
        const ChatRealtimeSessionFailure.forbidden(),
      ),
      policySubprotocolMismatch: () => _patchRealtimeSessionFailure(
        const ChatRealtimeSessionFailure.forbidden(),
      ),
      policyCloseCode: (_) => _patchRealtimeSessionFailure(
        const ChatRealtimeSessionFailure.forbidden(),
      ),
      tooManyConnectionsTerminal: () => _patchRealtimeSessionFailure(
        const ChatRealtimeSessionFailure.connectionLimitExceeded(),
      ),
      tooManyConnections: () => _patchState(
        (s) => s.copyWith(
          realtimeServiceFailure: const WsServiceFailure.tooManyConnections(),
        ),
      ),
      protocolBroken: () => throw StateError(
        'protocolBroken is handled in _handleWsServiceFailure',
      ),
    );
  }

  void _requestWsOverflowRefresh() {
    if (_wsOverflowRefreshInFlight) {
      return;
    }
    _wsOverflowRefreshInFlight = true;
    unawaited(
      refresh().whenComplete(() {
        _wsOverflowRefreshInFlight = false;
      }),
    );
  }

  void _onWsTaskStatus(WsTaskStatusEvent e) {
    if (e.projectId != projectId || e.taskId.isEmpty) {
      return;
    }
    final cur = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (cur == null) {
      return;
    }
    if (!cur.messages.any((m) => m.linkedTaskIds.contains(e.taskId))) {
      return;
    }

    final patch = LinkedTaskSnapshotPatch.fromWsTaskStatus(e);
    final next = <ConversationMessageModel>[];
    for (final m in cur.messages) {
      if (!m.linkedTaskIds.contains(e.taskId)) {
        next.add(m);
        continue;
      }
      next.add(applyLinkedSnapshotPatchToMessage(m, e.taskId, patch));
    }

    _patchState((s) => s.copyWith(messages: next));
  }

  void _handleWsServiceFailure(WsServiceFailure failure) {
    switch (failure) {
      case WsServiceFailureTransient():
        _patchState(
          (s) => s.copyWith(realtimeServiceFailure: failure),
        );
      case WsServiceFailureProtocolBroken():
        _patchState(
          (s) => s.copyWith(
            realtimeServiceFailure: const WsServiceFailure.protocolBroken(),
          ),
        );
      default:
        _applyTerminalServiceFailure(failure);
    }
  }

  void _onWsClientEvent(WsClientEvent ev) {
    switch (ev) {
      case WsClientEventServiceFailure(:final failure):
        _handleWsServiceFailure(failure);
        return;
      case WsClientEventAuthFailure():
        _patchRealtimeSessionFailure(
          const ChatRealtimeSessionFailure.authenticationLost(),
        );
        return;
      case WsClientEventSubprotocolMismatch():
      case WsClientEventParseError():
        _clearRealtimeTransientFailure();
        return;
      case WsClientEventServer(:final event):
        _clearRealtimeTransientFailure();
        event.when(
          taskStatus: _onWsTaskStatus,
          taskMessage: (_) {},
          agentLog: (_) {},
          error: (err) {
            if (err.projectId != projectId) {
              return;
            }
            if (err.needsRestRefetch) {
              _requestWsOverflowRefresh();
            }
          },
          unknown: (_) {},
        );
        return;
    }
  }

  /// Повторная загрузка метаданных и первой страницы истории.
  ///
  /// Pending по незавершённым отправкам сохраняются; [sendMessage] не отменяется.
  Future<void> refresh() async {
    _historyCancelToken?.cancel();
    _historyCancelToken = CancelToken();
    _sessionEpoch++;

    final prev = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    final pending = prev?.pendingByClientId ?? const <String, PendingMessage>{};
    state = AsyncData(
      ChatState(
        conversation: prev?.conversation,
        messages: const [],
        isLoadingInitial: true,
        isLoadingOlder: false,
        hasMoreOlder: false,
        olderOffset: 0,
        pendingByClientId: pending,
        realtimeServiceFailure: prev?.realtimeServiceFailure,
        realtimeSessionFailure: prev?.realtimeSessionFailure,
      ),
    );

    await _loadInitial();
  }

  Future<void> _loadInitial() async {
    final epoch = _sessionEpoch;
    final token = _historyCancelToken;
    final cid = conversationId;
    if (token == null) {
      return;
    }

    final repo = ref.read(conversationRepositoryProvider);

    try {
      final conversation = await repo.getConversation(
        cid,
        cancelToken: token,
      );
      if (epoch != _sessionEpoch) {
        return;
      }
      if (conversation.projectId != projectId) {
        throw StateError(
          'Conversation ${conversation.id} belongs to project '
          '${conversation.projectId}, expected $projectId',
        );
      }

      _patchState((s) => s.copyWith(conversation: conversation));

      final page = await repo.getMessages(
        cid,
        limit: _initialPageLimit,
        offset: 0,
        cancelToken: token,
      );
      if (epoch != _sessionEpoch) {
        return;
      }

      final cur = switch (state) {
        AsyncData<ChatState>(:final value) => value,
        _ => null,
      };
      if (cur == null) {
        return;
      }
      final idsBefore = cur.messages.map((m) => m.id).toSet();
      final merged = _mergeMessagesCanonical(cur.messages, page.messages);
      final hasMore = _hasMoreAfterPage(
        response: page,
        idsBeforeMerge: idsBefore,
      );

      _patchState(
        (s) => s.copyWith(
          messages: merged,
          isLoadingInitial: false,
          hasMoreOlder: hasMore,
          olderOffset: s.olderOffset + page.messages.length,
          realtimeServiceFailure: null,
        ),
      );
    } on ConversationCancelledException {
      if (epoch != _sessionEpoch) {
        return;
      }
    } catch (e, st) {
      if (epoch != _sessionEpoch) {
        return;
      }
      state = AsyncError(e, st);
    }
  }

  void _patchState(ChatState Function(ChatState s) fn) {
    final v = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (v == null) {
      return;
    }
    state = AsyncData(fn(v));
  }

  /// Подгрузка более старых сообщений (offset-пагинация).
  Future<void> loadOlder() {
    final inflight = _olderInFlight;
    if (inflight != null) {
      return inflight;
    }

    final s = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (s == null ||
        !s.hasMoreOlder ||
        s.isLoadingOlder ||
        s.isLoadingInitial) {
      return Future.value();
    }

    final epoch = _sessionEpoch;
    final token = _historyCancelToken;
    final cid = conversationId;
    if (token == null) {
      return Future.value();
    }

    final f = _loadOlderImpl(epoch: epoch, token: token, cid: cid);
    _olderInFlight = f;
    return f.whenComplete(() {
      if (identical(_olderInFlight, f)) {
        _olderInFlight = null;
      }
    });
  }

  Future<void> _loadOlderImpl({
    required int epoch,
    required CancelToken token,
    required String cid,
  }) async {
    final repo = ref.read(conversationRepositoryProvider);
    final cur = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (cur == null) {
      return;
    }

    _patchState((s) => s.copyWith(isLoadingOlder: true));

    try {
      final page = await repo.getMessages(
        cid,
        limit: _olderPageLimit,
        offset: cur.olderOffset,
        cancelToken: token,
      );
      if (epoch != _sessionEpoch) {
        return;
      }

      final afterStart = switch (state) {
        AsyncData<ChatState>(:final value) => value,
        _ => null,
      };
      if (afterStart == null) {
        return;
      }
      final idsBefore = afterStart.messages.map((m) => m.id).toSet();
      final merged = _mergeMessagesCanonical(afterStart.messages, page.messages);
      final hasMore = _hasMoreAfterPage(
        response: page,
        idsBeforeMerge: idsBefore,
      );

      _patchState(
        (s) => s.copyWith(
          messages: merged,
          isLoadingOlder: false,
          hasMoreOlder: hasMore,
          olderOffset: s.olderOffset + page.messages.length,
        ),
      );
    } on ConversationCancelledException {
      if (epoch == _sessionEpoch) {
        _patchState((s) => s.copyWith(isLoadingOlder: false));
      }
    } on ConversationNotFoundException catch (e, st) {
      if (epoch != _sessionEpoch) {
        return;
      }
      _patchState((s) => s.copyWith(isLoadingOlder: false));
      state = AsyncError(e, st);
    } catch (e, st) {
      if (epoch != _sessionEpoch) {
        return;
      }
      _patchState((s) => s.copyWith(isLoadingOlder: false));
      // Ошибка пагинации «старее» не сносит уже показанный чат (см. §11.4 тест #15).
      Error.throwWithStackTrace(e, st);
    }
  }

  /// Отправка сообщения; при транзиентной ошибке — запись в [ChatState.pendingByClientId].
  Future<ChatSendOutcome> send(String raw) async {
    final trimmed = raw.trim();
    if (trimmed.isEmpty) {
      return ChatSendOutcome.ignored;
    }

    final sessionBlocked = switch (state) {
      AsyncData<ChatState>(:final value) => value.realtimeSessionFailure != null,
      _ => false,
    };
    if (sessionBlocked) {
      return ChatSendOutcome.blockedByRealtimeSession;
    }

    final clientMessageId = generateClientMessageId();
    final cid = conversationId;
    final repo = ref.read(conversationRepositoryProvider);
    final now = DateTime.now();

    try {
      final result = await repo.sendMessage(
        cid,
        SendMessageRequest(content: trimmed),
        clientMessageId: clientMessageId,
        cancelToken: null,
      );
      _onSendSuccess(clientMessageId, result);
      return ChatSendOutcome.completed;
    } on ConversationCancelledException {
      _removePending(clientMessageId);
      return ChatSendOutcome.completed;
    } catch (e) {
      if (_isTransientSendFailure(e)) {
        _putPending(
          clientMessageId,
          PendingMessage(
            clientMessageId: clientMessageId,
            content: trimmed,
            lastError: e,
            attempts: 1,
            lastAttemptAt: now,
          ),
        );
        return ChatSendOutcome.completed;
      }
      _removePending(clientMessageId);
      rethrow;
    }
  }

  /// Повтор отправки с тем же [clientMessageId] и телом из pending.
  Future<ChatSendOutcome> retrySend(String clientMessageId) async {
    final s = switch (state) {
      AsyncData<ChatState>(:final value) => value,
      _ => null,
    };
    if (s?.realtimeSessionFailure != null) {
      return ChatSendOutcome.blockedByRealtimeSession;
    }
    final pending = s?.pendingByClientId[clientMessageId];
    if (pending == null) {
      return ChatSendOutcome.ignored;
    }

    final cid = conversationId;
    final repo = ref.read(conversationRepositoryProvider);
    final now = DateTime.now();
    final next = pending.copyWith(
      attempts: pending.attempts + 1,
      lastAttemptAt: now,
    );
    _putPending(clientMessageId, next);

    try {
      final result = await repo.sendMessage(
        cid,
        SendMessageRequest(content: pending.content),
        clientMessageId: clientMessageId,
        cancelToken: null,
      );
      _onSendSuccess(clientMessageId, result);
      return ChatSendOutcome.completed;
    } on ConversationCancelledException {
      _putPending(clientMessageId, pending);
      return ChatSendOutcome.completed;
    } catch (e) {
      if (_isTransientSendFailure(e)) {
        _putPending(
          clientMessageId,
          next.copyWith(lastError: e),
        );
        return ChatSendOutcome.completed;
      }
      _removePending(clientMessageId);
      rethrow;
    }
  }

  /// Убирает pending и вставляет [result.message] в ленту.
  ///
  /// Если провайдер уже в [AsyncError] (например, после фатального [refresh]),
  /// [_patchState] не меняет состояние — ответ успешной отправки **не**
  /// отобразится в ленте до следующего успешного входа в чат / [refresh].
  void _onSendSuccess(String clientMessageId, SendMessageResult result) {
    _removePending(clientMessageId);
    _patchState(
      (s) => s.copyWith(
        messages: _mergeMessagesCanonical(s.messages, [result.message]),
      ),
    );
  }

  // Полная копия Map на каждое изменение — ок для малого числа pending;
  // при узком горячем пути см. fast_immutable_collections / unmodifiable view.
  void _putPending(String clientMessageId, PendingMessage pending) {
    _patchState(
      (s) => s.copyWith(
        pendingByClientId: Map<String, PendingMessage>.from(s.pendingByClientId)
          ..[clientMessageId] = pending,
      ),
    );
  }

  void _removePending(String clientMessageId) {
    _patchState(
      (s) {
        if (!s.pendingByClientId.containsKey(clientMessageId)) {
          return s;
        }
        final next = Map<String, PendingMessage>.from(s.pendingByClientId)
          ..remove(clientMessageId);
        return s.copyWith(pendingByClientId: next);
      },
    );
  }

  /// Входная точка для реалтайма (11.9): upsert по id с пересортировкой.
  void applyIncomingMessage(ConversationMessageModel message) {
    if (message.conversationId != conversationId) {
      return;
    }
    _patchState(
      (s) => s.copyWith(
        messages: _mergeMessagesCanonical(s.messages, [message]),
      ),
    );
  }
}
