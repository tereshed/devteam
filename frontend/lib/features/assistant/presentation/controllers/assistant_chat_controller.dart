import 'dart:async';

import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_session_picker.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_chat_controller.g.dart';

/// Состояние ассистент-чата правой панели (Sprint 21 §10/§12 frontend).
///
/// Содержит активную сессию, её историю сообщений (ASC по `createdAt`),
/// флаги busy/sending для UI input-disable, последнее
/// [WsAssistantConfirmRequestEvent] для inline-confirm-карточки и
/// «consume-once» [WsAssistantNavigateEvent] для `GoRouter.go`.
class AssistantChatState {
  const AssistantChatState({
    this.currentSessionId,
    this.session,
    this.messages = const <AssistantMessageModel>[],
    this.loadingHistory = false,
    this.hasMore = false,
    this.nextBeforeCreatedAt,
    this.nextBeforeId,
    this.sending = false,
    this.creatingSession = false,
    this.error,
    this.pendingConfirm,
    this.pendingNavigate,
  });

  final String? currentSessionId;

  /// Полные данные активной сессии (busy, busy_since, pending_tool_call_id).
  /// Источник правды для UI input-disable.
  final AssistantSessionModel? session;

  /// История сообщений активной сессии, **ASC по createdAt** — фронт инвертирует
  /// порядок относительно REST (бэкенд отдаёт DESC, мы переворачиваем для списка).
  final List<AssistantMessageModel> messages;

  /// Идёт начальная подгрузка истории / следующей страницы.
  final bool loadingHistory;

  /// Есть ли страница старше — для бесконечного scroll'а вверх.
  final bool hasMore;
  final DateTime? nextBeforeCreatedAt;
  final String? nextBeforeId;

  /// Идёт `POST /messages` (оптимистичный insert уже произошёл, но запрос ещё
  /// не вернулся). Отдельный флаг от `session.busy`, чтобы UI блокировал
  /// input сразу после клика, не дожидаясь WS-апдейта.
  final bool sending;

  /// Идёт `POST /sessions` (создание первой сессии).
  final bool creatingSession;

  /// Последняя ошибка REST/WS (для SnackBar / inline message).
  final AssistantRepositoryException? error;

  /// Pending confirm-request — UI рендерит inline-карточку в конце списка
  /// (Approve/Deny). Очищается после успешного `POST /confirm`.
  final WsAssistantConfirmRequestEvent? pendingConfirm;

  /// Consume-once navigate event для GoRouter. Виджет, увидев непустое
  /// значение, делает `context.go(route)` и зовёт [consumeNavigate].
  final WsAssistantNavigateEvent? pendingNavigate;

  /// busy = session.busy ИЛИ идёт sending/creatingSession. Удобный getter,
  /// чтобы виджет не повторял булевую логику.
  bool get isBusy =>
      sending || creatingSession || (session?.busy ?? false);

  AssistantChatState copyWith({
    String? currentSessionId,
    AssistantSessionModel? session,
    List<AssistantMessageModel>? messages,
    bool? loadingHistory,
    bool? hasMore,
    Object? nextBeforeCreatedAt = _sentinel,
    Object? nextBeforeId = _sentinel,
    bool? sending,
    bool? creatingSession,
    Object? error = _sentinel,
    Object? pendingConfirm = _sentinel,
    Object? pendingNavigate = _sentinel,
  }) {
    return AssistantChatState(
      currentSessionId: currentSessionId ?? this.currentSessionId,
      session: session ?? this.session,
      messages: messages ?? this.messages,
      loadingHistory: loadingHistory ?? this.loadingHistory,
      hasMore: hasMore ?? this.hasMore,
      nextBeforeCreatedAt: identical(nextBeforeCreatedAt, _sentinel)
          ? this.nextBeforeCreatedAt
          : nextBeforeCreatedAt as DateTime?,
      nextBeforeId: identical(nextBeforeId, _sentinel)
          ? this.nextBeforeId
          : nextBeforeId as String?,
      sending: sending ?? this.sending,
      creatingSession: creatingSession ?? this.creatingSession,
      error: identical(error, _sentinel)
          ? this.error
          : error as AssistantRepositoryException?,
      pendingConfirm: identical(pendingConfirm, _sentinel)
          ? this.pendingConfirm
          : pendingConfirm as WsAssistantConfirmRequestEvent?,
      pendingNavigate: identical(pendingNavigate, _sentinel)
          ? this.pendingNavigate
          : pendingNavigate as WsAssistantNavigateEvent?,
    );
  }

  static const Object _sentinel = Object();
}

/// Контроллер ассистент-чата (Sprint 21 §10 frontend).
///
/// Делает:
/// - REST: создание/выбор сессии, подгрузка истории (курсорная пагинация),
///   отправка сообщений (с идемпотентностью), подтверждение destructive
///   операций;
/// - WS: подписан на `assistant.*` и мержит события в локальный список;
///   фильтрует по `currentSessionId` (кроме `assistant.navigate`, который
///   user-scoped и применяется всегда).
@Riverpod(keepAlive: true)
class AssistantChatController extends _$AssistantChatController {
  StreamSubscription<WsClientEvent>? _wsSubscription;
  Timer? _pollingTimer;
  bool _pollingInFlight = false;

  @override
  AssistantChatState build() {
    final ws = ref.read(webSocketServiceProvider);
    _wsSubscription = ws.events.listen(_onWsClientEvent);
    ref.onDispose(() {
      unawaited(_wsSubscription?.cancel());
      _wsSubscription = null;
      _pollingTimer?.cancel();
    });

    // Наблюдаем за активным проектом. При смене проекта Riverpod пересоздаст этот контроллер.
    ref.watch(activeProjectIdProvider);

    return const AssistantChatState();
  }

  // ─────────────────────────── Session management ───────────────────────────

  /// Гарантирует, что активная сессия есть. Если нет — пытается взять самую
  /// свежую из ListSessions, иначе создаёт новую.
  ///
  /// Возвращает sessionId или бросает [AssistantRepositoryException].
  Future<String> ensureSession() async {
    final activeProjectId = ref.read(activeProjectIdProvider);
    final current = state.currentSessionId;
    // Reuse только если scope текущей сессии совпадает с активным проектом.
    // Контроллер keepAlive: если он не пересоздался при входе в проект (или
    // глобальная сессия осталась с прошлого экрана), early-return без проверки
    // scope «приклеивал» бы глобальный чат внутри проекта (инцидент: tasks →
    // Дашборд проекта → ассистент глобальный). При mismatch проваливаемся в
    // переподбор сессии правильного scope ниже.
    if (current != null && state.session?.projectId == activeProjectId) {
      return current;
    }

    if (state.creatingSession) {
      // Ждём завершения уже идущего create — не плодим параллельных запросов.
      // Простая стратегия: бросаем «ещё не готово», UI повторит попытку.
      throw AssistantApiException(
        'session is being created',
        isNetworkTransportError: false,
      );
    }

    state = state.copyWith(creatingSession: true, error: null);
    try {
      final repo = ref.read(assistantRepositoryProvider);
      final projectId = activeProjectId;
      final sessions = await repo.listSessions(limit: 1, projectId: projectId);
      AssistantSessionModel session;
      if (sessions.sessions.isNotEmpty &&
          sessions.sessions.first.status == assistantSessionStatusActive) {
        session = sessions.sessions.first;
      } else {
        session = await repo.createSession(projectId: projectId);
        ref.invalidate(assistantSessionsListProvider);
      }
      await _selectSession(session);
      return session.id;
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(creatingSession: false, error: e);
      rethrow;
    } finally {
      if (state.creatingSession) {
        state = state.copyWith(creatingSession: false);
      }
    }
  }

  /// Явный switch на конкретную сессию (выбор из dropdown / session picker).
  Future<void> selectSession(String sessionId) async {
    if (state.currentSessionId == sessionId) return;
    final repo = ref.read(assistantRepositoryProvider);
    try {
      final session = await repo.getSession(sessionId);
      await _selectSession(session);
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(error: e);
      rethrow;
    }
  }

  /// Создаёт новую пустую сессию и переключается на неё.
  Future<String> startNewSession() async {
    state = state.copyWith(creatingSession: true, error: null);
    try {
      final repo = ref.read(assistantRepositoryProvider);
      final projectId = ref.read(activeProjectIdProvider);
      final session = await repo.createSession(projectId: projectId);
      ref.invalidate(assistantSessionsListProvider);
      await _selectSession(session);
      return session.id;
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(creatingSession: false, error: e);
      rethrow;
    } finally {
      if (state.creatingSession) {
        state = state.copyWith(creatingSession: false);
      }
    }
  }

  /// Архивирует сессию. Если архивируем активную — сбрасываем выбор.
  Future<void> archiveSession(String sessionId) async {
    final repo = ref.read(assistantRepositoryProvider);
    try {
      await repo.archiveSession(sessionId);
      ref.invalidate(assistantSessionsListProvider);
      if (state.currentSessionId == sessionId) {
        state = const AssistantChatState();
      }
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(error: e);
      rethrow;
    }
  }

  Future<void> _selectSession(AssistantSessionModel session) async {
    // Гард scope: сессия чужого scope (глобальная внутри проекта, чужой проект,
    // проектная на глобальном экране) не может стать текущей — ассистент молча
    // терял бы PROJECT CONTEXT и вёл себя «глобально» внутри проекта (инцидент:
    // «о чём у нас проект» → project_list + вопрос «какой проект?»). Mismatch →
    // подменяем на свежую сессию правильного scope (или создаём).
    final activeProjectId = ref.read(activeProjectIdProvider);
    if (session.projectId != activeProjectId) {
      final repo = ref.read(assistantRepositoryProvider);
      final sessions =
          await repo.listSessions(limit: 1, projectId: activeProjectId);
      if (sessions.sessions.isNotEmpty &&
          sessions.sessions.first.status == assistantSessionStatusActive) {
        session = sessions.sessions.first;
      } else {
        session = await repo.createSession(projectId: activeProjectId);
        ref.invalidate(assistantSessionsListProvider);
      }
    }

    // Сбрасываем всё, что было от предыдущей сессии (включая pendingConfirm —
    // иначе при switch'е остался бы зависший диалог).
    _pollingTimer?.cancel();
    _pollingTimer = null;
    _pollingInFlight = false;

    state = AssistantChatState(
      currentSessionId: session.id,
      session: session,
      loadingHistory: true,
    );
    await _loadInitialHistory(session.id);
    if (state.session?.busy == true) {
      _startPollingIfBusy();
    }
  }

  Future<void> _loadInitialHistory(String sessionId) async {
    final repo = ref.read(assistantRepositoryProvider);
    try {
      final page = await repo.getMessages(sessionId);
      // Бэкенд отдаёт DESC; чат отображаем ASC.
      final ordered = page.messages.reversed.toList(growable: false);
      state = state.copyWith(
        messages: ordered,
        loadingHistory: false,
        hasMore: page.hasMore,
        nextBeforeCreatedAt: page.nextBeforeCreatedAt,
        nextBeforeId: page.nextBeforeId,
      );
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(loadingHistory: false, error: e);
    }
  }

  /// Подгружает старшую страницу истории (бесконечный scroll вверх).
  Future<void> loadOlder() async {
    final sessionId = state.currentSessionId;
    if (sessionId == null ||
        state.loadingHistory ||
        !state.hasMore ||
        state.nextBeforeCreatedAt == null ||
        state.nextBeforeId == null) {
      return;
    }
    state = state.copyWith(loadingHistory: true);
    final repo = ref.read(assistantRepositoryProvider);
    try {
      final page = await repo.getMessages(
        sessionId,
        beforeCreatedAt: state.nextBeforeCreatedAt,
        beforeId: state.nextBeforeId,
      );
      final ordered = page.messages.reversed.toList(growable: false);
      // Новые (старые по времени) идут ВВЕРХ списка.
      state = state.copyWith(
        messages: [...ordered, ...state.messages],
        loadingHistory: false,
        hasMore: page.hasMore,
        nextBeforeCreatedAt: page.nextBeforeCreatedAt,
        nextBeforeId: page.nextBeforeId,
      );
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(loadingHistory: false, error: e);
    }
  }

  // ─────────────────────────── Messaging ───────────────────────────

  /// Отправка user-сообщения. Если сессии ещё нет — создаёт её первой.
  /// Идемпотентность — через сгенерированный UUIDv4 (header X-Client-Message-ID).
  Future<void> sendMessage(String content) async {
    final trimmed = content.trim();
    if (trimmed.isEmpty) return;
    if (state.isBusy) return;

    String sessionId;
    try {
      sessionId = await ensureSession();
    } on AssistantRepositoryException {
      // Ошибка уже записана в state.error внутри ensureSession.
      return;
    }

    final clientMessageId = generateClientMessageId();
    state = state.copyWith(sending: true, error: null);

    final repo = ref.read(assistantRepositoryProvider);
    try {
      final resp = await repo.sendMessage(
        sessionId,
        content: trimmed,
        clientMessageId: clientMessageId,
      );
      // Upsert по id: для нового сообщения — insert, для duplicate (повтор
      // POST с тем же client_message_id) — перезапись существующего тем же
      // содержимым. Ветвление по `resp.duplicate` тут не нужно — upsert
      // идемпотентен. Сам флаг `duplicate` влияет только на typing-индикатор
      // (UI смотрит на него отдельно через state.sending).
      _appendOrUpdateMessage(resp.message);
      if (state.session != null) {
        state = state.copyWith(session: state.session!.copyWith(busy: true));
      }
      state = state.copyWith(sending: false);
      _startPollingIfBusy();
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(sending: false, error: e);
    }
  }

  // ─────────────────────────── Confirm ───────────────────────────

  Future<void> confirmToolCall({
    required String toolCallId,
    required bool approved,
  }) async {
    final sessionId = state.currentSessionId;
    if (sessionId == null) return;

    state = state.copyWith(error: null);
    final repo = ref.read(assistantRepositoryProvider);
    try {
      await repo.confirmToolCall(
        sessionId,
        toolCallId: toolCallId,
        approved: approved,
      );
      // Снимаем pendingConfirm только при успехе. Параллельный confirm
      // (already_confirmed) тоже считаем «UI больше не должен показывать
      // карточку» — рассматриваем как успех.
      state = state.copyWith(
        pendingConfirm: null,
        session: state.session?.copyWith(busy: true),
      );
      _startPollingIfBusy();
    } on AssistantAlreadyConfirmedException {
      state = state.copyWith(
        pendingConfirm: null,
        session: state.session?.copyWith(busy: true),
      );
      _startPollingIfBusy();
    } on AssistantRepositoryException catch (e) {
      state = state.copyWith(error: e);
    }
  }

  /// UI вызывает после context.go(route) — снимает «consume-once» событие.
  void consumeNavigate() {
    if (state.pendingNavigate == null) return;
    state = state.copyWith(pendingNavigate: null);
  }

  /// Снимает текущую ошибку (после показа SnackBar).
  void clearError() {
    if (state.error == null) return;
    state = state.copyWith(error: null);
  }

  // ─────────────────────────── WS handlers ───────────────────────────

  void _onWsClientEvent(WsClientEvent ev) {
    if (ev is! WsClientEventServer) return;
    ev.event.maybeMap(
      assistantSessionUpdated: (e) {
        ref.invalidate(assistantSessionsListProvider);
        if (!_matchesCurrent(e.value.sessionId)) return null;
        _applySessionUpdated(e.value);
        return null;
      },
      assistantMessage: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        _applyMessageEvent(e.value);
        return null;
      },
      assistantToolCall: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        _applyToolCallEvent(e.value);
        return null;
      },
      assistantToolResult: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        _applyToolResultEvent(e.value);
        return null;
      },
      assistantConfirmRequest: (e) {
        if (!_matchesCurrent(e.value.sessionId)) return null;
        state = state.copyWith(pendingConfirm: e.value);
        return null;
      },
      assistantNavigate: (e) {
        // navigate — user-scoped, без session_id. Применяем всегда.
        state = state.copyWith(pendingNavigate: e.value);
        return null;
      },
      orElse: () => null,
    );
  }

  bool _matchesCurrent(String sessionId) {
    final current = state.currentSessionId;
    return current != null && current == sessionId;
  }

  void _applySessionUpdated(WsAssistantSessionUpdatedEvent ev) {
    final s = state.session;
    if (s == null) return;
    final titleChanged = ev.title != null && ev.title != s.title;
    final updated = s.copyWith(
      title: ev.title ?? s.title,
      status: ev.status,
      busy: ev.busy,
      lastMessageAt: ev.lastMessageAt ?? s.lastMessageAt,
      updatedAt: ev.updatedAt,
    );
    state = state.copyWith(session: updated);
    if (titleChanged) {
      ref.invalidate(assistantSessionsListProvider);
    }
    if (updated.busy) {
      _startPollingIfBusy();
    }
  }

  void _applyMessageEvent(WsAssistantMessageEvent ev) {
    // assistant.message — это row для role user/assistant/system. Конструируем
    // partial-модель и upsert'им. tool_arguments/tool_result в этом событии
    // НЕ передаются (бэкенд шлёт их отдельно через tool_call/tool_result).
    final msg = AssistantMessageModel(
      id: ev.messageId,
      sessionId: ev.sessionId,
      role: ev.role,
      content: ev.content,
      toolCallId: ev.toolCallId,
      toolName: ev.toolName,
      toolArguments: null,
      toolResult: null,
      clientMessageId: null,
      createdAt: ev.createdAt,
    );
    _appendOrUpdateMessage(msg);
  }

  void _applyToolCallEvent(WsAssistantToolCallEvent ev) {
    // Ищем уже добавленную assistant-row по toolCallId — это row из
    // OnAssistantMessage на бэкенде. Если нашли — патчим arguments.
    // Если ещё не дошёл assistant.message (race) — игнорируем: arguments
    // переедут позже через REST либо UI обойдётся без них.
    final idx = state.messages
        .indexWhere((m) => m.toolCallId != null && m.toolCallId == ev.toolCallId);
    if (idx < 0) return;
    final existing = state.messages[idx];
    final updated = existing.copyWith(
      toolName: existing.toolName ?? ev.toolName,
      toolArguments: ev.arguments,
    );
    final newList = [...state.messages]..[idx] = updated;
    state = state.copyWith(messages: newList);
  }

  void _applyToolResultEvent(WsAssistantToolResultEvent ev) {
    // Тут две row'и могут описывать пару call+result:
    //   1) assistant-row с tool_call_id (была emitнута OnAssistantMessage)
    //   2) tool-row, которую только что создал OnToolResult — её мы вставляем
    //      в список как НОВОЕ сообщение, чтобы UI показал «✅ result».
    // Если же tool-row с этим tool_call_id уже есть (REST на холодный старт),
    // просто патчим result.
    final existingToolIdx = state.messages.indexWhere(
      (m) =>
          m.role == assistantMessageRoleTool &&
          m.toolCallId != null &&
          m.toolCallId == ev.toolCallId,
    );
    // Бэкенд шлёт `status` отдельным полем WS-события (ok/forbidden/denied/
    // error/truncated/pending), а `result` — сырой payload MCP-вызова без
    // самостоятельного поля «status». Чтобы виджет (AssistantToolCallCard)
    // мог отрисовать статус-бейдж без дополнительной prop'ы, оборачиваем
    // status внутрь toolResult-мапы под зарезервированным ключом `status`.
    // Это «синтетическое» поле — REST history его НЕ возвращает (там оно
    // живёт в logs/audit, не в БД); для REST-row бейдж не показываем —
    // ровно то поведение, которого хочет UX (status релевантен только в
    // live-петле, по холодной истории он не нужен).
    final mergedResult = <String, dynamic>{
      'status': ev.status,
      ...ev.result,
    };
    if (existingToolIdx >= 0) {
      final existing = state.messages[existingToolIdx];
      final updated = existing.copyWith(
        toolName: existing.toolName ?? ev.toolName,
        toolResult: mergedResult,
      );
      final newList = [...state.messages]..[existingToolIdx] = updated;
      state = state.copyWith(messages: newList);
      return;
    }
    // Вставляем новую tool-row. ID не приходит в WS-событии (messageId
    // optional) — используем synthetic id, чтобы дедупликация по id внутри
    // _appendOrUpdateMessage работала и по REST'у (id из БД) её перезатёрло.
    final syntheticId = ev.messageId ?? 'tool:${ev.toolCallId}';
    final msg = AssistantMessageModel(
      id: syntheticId,
      sessionId: ev.sessionId,
      role: assistantMessageRoleTool,
      content: null,
      toolCallId: ev.toolCallId,
      toolName: ev.toolName,
      toolArguments: null,
      toolResult: mergedResult,
      clientMessageId: null,
      // ts события — лучший доступный timestamp; REST позже отдаст
      // настоящий created_at, и сообщение перезапишется.
      createdAt: ev.ts,
    );
    _appendOrUpdateMessage(msg);
  }

  /// Upsert по `id`: если уже есть — заменяем, иначе вставляем в конец
  /// (естественный порядок ASC по createdAt; backend гарантирует
  /// монотонность created_at в рамках одной сессии).
  void _appendOrUpdateMessage(AssistantMessageModel msg) {
    final idx = state.messages.indexWhere((m) => m.id == msg.id);
    if (idx >= 0) {
      final existing = state.messages[idx];
      // Сохраняем уже накопленные поля (toolArguments/toolResult), если
      // новое событие их не несёт — иначе WS-апдейт без аргументов
      // затёр бы патч от tool_call'а.
      final merged = existing.copyWith(
        role: msg.role,
        content: msg.content ?? existing.content,
        toolCallId: msg.toolCallId ?? existing.toolCallId,
        toolName: msg.toolName ?? existing.toolName,
        toolArguments: msg.toolArguments ?? existing.toolArguments,
        toolResult: msg.toolResult ?? existing.toolResult,
        clientMessageId: msg.clientMessageId ?? existing.clientMessageId,
        createdAt: msg.createdAt,
      );
      final newList = [...state.messages]..[idx] = merged;
      state = state.copyWith(messages: newList);
      return;
    }
    // Поддерживаем ASC-порядок: ищем позицию вставки по createdAt.
    final list = state.messages;
    final insertAt = list.lastIndexWhere((m) => !m.createdAt.isAfter(msg.createdAt)) + 1;
    final newList = [...list]..insert(insertAt, msg);
    state = state.copyWith(messages: newList);
  }

  void _startPollingIfBusy() {
    if (_pollingTimer != null) return;
    final sessionId = state.currentSessionId;
    if (sessionId == null) return;
    if (state.session?.busy != true) return;

    _pollingTimer = Timer.periodic(const Duration(seconds: 2), (timer) async {
      final currentId = state.currentSessionId;
      if (currentId != sessionId || state.session?.busy != true) {
        timer.cancel();
        if (_pollingTimer == timer) {
          _pollingTimer = null;
        }
        return;
      }

      if (_pollingInFlight) return;
      _pollingInFlight = true;

      try {
        final repo = ref.read(assistantRepositoryProvider);
        final futureSession = repo.getSession(sessionId);
        final futureMessages = repo.getMessages(sessionId);

        final updatedSession = await futureSession;
        final messagesResponse = await futureMessages;

        if (state.currentSessionId != sessionId) {
          timer.cancel();
          if (_pollingTimer == timer) {
            _pollingTimer = null;
          }
          return;
        }

        // Upsert all fetched messages
        for (final msg in messagesResponse.messages) {
          _appendOrUpdateMessage(msg);
        }

        // Handle confirmation prompts based on pendingToolCallId
        final pendingId = updatedSession.pendingToolCallId;
        if (pendingId != null && pendingId.isNotEmpty) {
          if (state.pendingConfirm == null ||
              state.pendingConfirm!.toolCallId != pendingId) {
            final toolMsgIdx =
                state.messages.indexWhere((m) => m.toolCallId == pendingId);
            if (toolMsgIdx >= 0) {
              final toolMsg = state.messages[toolMsgIdx];
              final confirmEvent = WsAssistantConfirmRequestEvent(
                ts: toolMsg.createdAt,
                v: 1,
                userId: updatedSession.userId,
                sessionId: sessionId,
                toolCallId: pendingId,
                toolName: toolMsg.toolName ?? '',
                arguments: toolMsg.toolArguments ?? const <String, dynamic>{},
                summary: toolMsg.content,
              );
              state = state.copyWith(
                pendingConfirm: confirmEvent,
                session: updatedSession,
              );
            } else {
              state = state.copyWith(session: updatedSession);
            }
          } else {
            state = state.copyWith(session: updatedSession);
          }
        } else {
          state = state.copyWith(
            session: updatedSession,
            pendingConfirm: null,
          );
        }

        if (!updatedSession.busy) {
          timer.cancel();
          if (_pollingTimer == timer) {
            _pollingTimer = null;
          }
        }
      } catch (e) {
        // Ignore errors to retry on the next interval
      } finally {
        _pollingInFlight = false;
      }
    });
  }
}

/// Helper для UI: группирует сообщения по `toolCallId`, чтобы рендерить
/// одну карточку «вызов tool + его результат» (UX план §8 frontend).
///
/// Возвращает кортежи `(assistantMsg, toolResultMsg?)` для каждого tool-call'а
/// и одиночные сообщения для не-tool ролей.
class AssistantMessageGroup {
  AssistantMessageGroup({
    required this.assistantMessage,
    this.toolResult,
  });

  final AssistantMessageModel assistantMessage;
  final AssistantMessageModel? toolResult;

  bool get isToolCall =>
      assistantMessage.toolCallId != null &&
      assistantMessage.toolCallId!.isNotEmpty;
}

List<AssistantMessageGroup> groupAssistantMessages(
  List<AssistantMessageModel> messages,
) {
  final result = <AssistantMessageGroup>[];
  // Индекс tool-row'ов по toolCallId для O(1) lookup при сборке пар.
  final toolResults = <String, AssistantMessageModel>{};
  for (final m in messages) {
    if (m.role == assistantMessageRoleTool &&
        m.toolCallId != null &&
        m.toolCallId!.isNotEmpty) {
      toolResults[m.toolCallId!] = m;
    }
  }
  final consumedToolIds = <String>{};
  for (final m in messages) {
    if (m.role == assistantMessageRoleTool) {
      // Tool-row'ы рендерим внутри tool-call карточки выше, отдельно — нет.
      continue;
    }
    if (m.toolCallId != null && m.toolCallId!.isNotEmpty) {
      final res = toolResults[m.toolCallId!];
      if (res != null) consumedToolIds.add(m.toolCallId!);
      result.add(
        AssistantMessageGroup(assistantMessage: m, toolResult: res),
      );
    } else {
      result.add(AssistantMessageGroup(assistantMessage: m));
    }
  }
  // Tool-row'ы без assistant-парной row (на холодный старт REST может
  // вернуть тулрезультат раньше, или backend пометит legacy) — показываем
  // отдельной карточкой, чтобы не терять данных.
  final orphans = messages.where(
    (m) =>
        m.role == assistantMessageRoleTool &&
        m.toolCallId != null &&
        m.toolCallId!.isNotEmpty &&
        !consumedToolIds.contains(m.toolCallId),
  );
  for (final orphan in orphans) {
    result.add(AssistantMessageGroup(assistantMessage: orphan));
  }
  // Финальная сортировка по createdAt assistant-row, чтобы orphans встали
  // на правильное место (а не в конце).
  result.sort((a, b) =>
      a.assistantMessage.createdAt.compareTo(b.assistantMessage.createdAt));
  return result;
}

