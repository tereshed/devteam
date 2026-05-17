import 'dart:convert';

import 'package:freezed_annotation/freezed_annotation.dart';

part 'websocket_events.freezed.dart';

// ---------------------------------------------------------------------------
// Auth (subprotocol negotiation)
// ---------------------------------------------------------------------------

/// Аутентификация WebSocket: **никогда** не используйте `Future<WsAuth?>`.
///
/// [toString] намеренно не содержит секретов (Freezed по умолчанию выводит поля).
@freezed
abstract class WsAuth with _$WsAuth {
  const WsAuth._();
  const factory WsAuth.bearer(String jwt) = WsAuthBearer;
  const factory WsAuth.apiKey(String secret) = WsAuthApiKey;
  const factory WsAuth.none() = WsAuthNone;

  @override
  String toString() => map(
        bearer: (_) => 'WsAuth.bearer(***)',
        apiKey: (_) => 'WsAuth.apiKey(***)',
        none: (_) => 'WsAuth.none()',
      );
}

typedef WsAuthProvider = Future<WsAuth> Function();

// ---------------------------------------------------------------------------
// Error codes (7.3) — case-sensitive, без toLowerCase
// ---------------------------------------------------------------------------

enum WsErrorCode {
  streamOverflow('stream_overflow'),
  taskNotFound('task_not_found'),
  internalError('internal_error'),
  forbidden('forbidden'),
  serverShutdown('server_shutdown');

  const WsErrorCode(this.jsonValue);
  final String jsonValue;

  static WsErrorCode? tryParse(String raw) {
    for (final v in WsErrorCode.values) {
      if (v.jsonValue == raw) {
        return v;
      }
    }
    return null;
  }
}

// ---------------------------------------------------------------------------
// Parse / protocol failures
// ---------------------------------------------------------------------------

@freezed
abstract class WsParseError with _$WsParseError {
  const factory WsParseError({
    required String message,
    String? detail,
  }) = _WsParseError;
}

@freezed
abstract class WsAuthFailure with _$WsAuthFailure {
  const factory WsAuthFailure({
    String? closeReason,
    int? closeCode,
  }) = _WsAuthFailure;
}

/// Событие рассогласования subprotocol. **[expected]** и **[received]** —
/// только схема/маскированные значения, без реального JWT/API key (11.2, security).
@freezed
abstract class WsSubprotocolMismatch with _$WsSubprotocolMismatch {
  const factory WsSubprotocolMismatch({
    required String expected,
    String? received,
  }) = _WsSubprotocolMismatch;
}

/// Литерал схемы для UI/логов (не передаёт секрет).
String wsSubprotocolSchemeExpected(WsAuth auth) {
  if (auth is WsAuthBearer) {
    return 'bearer.<jwt>';
  }
  if (auth is WsAuthApiKey) {
    return 'apikey.<secret>';
  }
  return '(none)';
}

/// Безопасное отображение negotiated subprotocol.
String wsSubprotocolReceivedForDisplay(String? negotiated) {
  if (negotiated == null || negotiated.isEmpty) {
    return '(null)';
  }
  if (negotiated.startsWith('bearer.')) {
    return 'bearer.***';
  }
  if (negotiated.startsWith('apikey.')) {
    return 'apikey.***';
  }
  return negotiated;
}

// ---------------------------------------------------------------------------
// Service-level failures (11.9) — отдельно от payload 7.3
// ---------------------------------------------------------------------------

/// Сбои уровня сервиса (сеть, политика, лимиты). Для **11.9**: один `switch` по вариантам,
/// без разбора строк.
///
/// **Лимит коннектов (4429):** сначала [WsServiceFailure.tooManyConnections] — одна
/// отложенная попытка переподключения. Если после неё снова **4429** в том же «эпизоде»
/// (см. счётчик в [WebSocketService]), приходит [WsServiceFailure.tooManyConnectionsTerminal]
/// — в UX показать блокировку до действия пользователя (не путать с первым событием).
@freezed
abstract class WsServiceFailure with _$WsServiceFailure {
  const factory WsServiceFailure.transient([Object? cause]) =
      WsServiceFailureTransient;
  const factory WsServiceFailure.authExpired() = WsServiceFailureAuthExpired;
  /// Сервер отклонил сессию по политике (7.3 `forbidden`).
  const factory WsServiceFailure.policyForbidden() =
      WsServiceFailurePolicyForbidden;
  /// Рассогласование Sec-WebSocket-Protocol после connect.
  const factory WsServiceFailure.policySubprotocolMismatch() =
      WsServiceFailurePolicySubprotocolMismatch;
  /// Закрытие с кодом политики (напр. **1008**).
  const factory WsServiceFailure.policyCloseCode(int code) =
      WsServiceFailurePolicyCloseCode;
  /// Второй **4429** подряд в одном «эпизоде» (см. счётчик в [WebSocketService]).
  const factory WsServiceFailure.tooManyConnectionsTerminal() =
      WsServiceFailureTooManyConnectionsTerminal;
  /// Первый **4429** в эпизоде: клиент планирует **одну** отложенную попытку (см. [WebSocketService]).
  const factory WsServiceFailure.tooManyConnections() =
      WsServiceFailureTooManyConnections;
  const factory WsServiceFailure.protocolBroken() =
      WsServiceFailureProtocolBroken;
}

// ---------------------------------------------------------------------------
// Server events (7.3 envelope)
// ---------------------------------------------------------------------------

@freezed
abstract class WsTaskStatusEvent with _$WsTaskStatusEvent {
  const factory WsTaskStatusEvent({
    required DateTime ts,
    required int v,
    required String projectId,
    required String taskId,
    String? parentTaskId,
    required String previousStatus,
    required String status,
    String? assignedAgentId,
    String? agentRole,
    String? errorMessage,
  }) = _WsTaskStatusEvent;
}

@freezed
abstract class WsTaskMessageEvent with _$WsTaskMessageEvent {
  const factory WsTaskMessageEvent({
    required DateTime ts,
    required int v,
    required String projectId,
    required String taskId,
    required String messageId,
    required String senderType,
    required String senderId,
    String? senderRole,
    required String messageType,
    required String content,
    @Default(<String, dynamic>{}) Map<String, dynamic> metadata,
  }) = _WsTaskMessageEvent;
}

@freezed
abstract class WsAgentLogEvent with _$WsAgentLogEvent {
  const factory WsAgentLogEvent({
    required DateTime ts,
    required int v,
    required String projectId,
    required String taskId,
    required String sandboxId,
    required String stream,
    required String line,
    required int seq,
    @Default(false) bool truncated,
  }) = _WsAgentLogEvent;
}

@freezed
abstract class WsErrorEvent with _$WsErrorEvent {
  const factory WsErrorEvent({
    required DateTime ts,
    required int v,
    required String projectId,
    required WsErrorCode code,
    required String message,
    @Default(<String, dynamic>{}) Map<String, dynamic> details,
    /// Для [WsErrorCode.streamOverflow] — сигнал REST refetch (11.3).
    @Default(false) bool needsRestRefetch,
  }) = _WsErrorEvent;
}

@freezed
abstract class WsUnknownEvent with _$WsUnknownEvent {
  const factory WsUnknownEvent({
    required String type,
    required DateTime ts,
    required int v,
    required String projectId,
    required Map<String, dynamic> data,
  }) = _WsUnknownEvent;
}

/// User-scoped событие: статус подключения внешней интеграции (LLM-провайдер,
/// Claude Code OAuth и т.п.). В envelope нет `project_id` — вместо него `user_id`.
/// Backend: `events.IntegrationConnectionChanged` → `Hub.SendToUser`
/// (см. dashboard-redesign §4a.4).
@freezed
abstract class WsIntegrationStatusEvent with _$WsIntegrationStatusEvent {
  const factory WsIntegrationStatusEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String provider,
    required WsIntegrationStatus status,
    String? reason,
    DateTime? connectedAt,
    DateTime? expiresAt,
  }) = _WsIntegrationStatusEvent;
}

// ---------------------------------------------------------------------------
// Assistant events (Sprint 21 §7). User-scoped: envelope несёт user_id,
// project_id отсутствует (кроме assistant.task_update, где он внутри data).
// Бэкенд: ws.MarshalAssistant*  → MarshalUserEnvelope → Hub.SendToUser.
// ---------------------------------------------------------------------------

/// Базовый набор общих полей user-scoped envelope'а ассистента.
/// Конкретные события расширяют его специфичной payload-частью.
@freezed
abstract class WsAssistantSessionUpdatedEvent
    with _$WsAssistantSessionUpdatedEvent {
  const factory WsAssistantSessionUpdatedEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String sessionId,
    String? title,
    required String status,
    required bool busy,
    DateTime? lastMessageAt,
    required DateTime updatedAt,
  }) = _WsAssistantSessionUpdatedEvent;
}

@freezed
abstract class WsAssistantMessageEvent with _$WsAssistantMessageEvent {
  const factory WsAssistantMessageEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String sessionId,
    required String messageId,
    required String role,
    String? content,
    String? toolCallId,
    String? toolName,
    required DateTime createdAt,
  }) = _WsAssistantMessageEvent;
}

@freezed
abstract class WsAssistantToolCallEvent with _$WsAssistantToolCallEvent {
  const factory WsAssistantToolCallEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String sessionId,
    String? messageId,
    required String toolCallId,
    required String toolName,
    @Default(<String, dynamic>{}) Map<String, dynamic> arguments,
  }) = _WsAssistantToolCallEvent;
}

@freezed
abstract class WsAssistantToolResultEvent with _$WsAssistantToolResultEvent {
  const factory WsAssistantToolResultEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String sessionId,
    String? messageId,
    required String toolCallId,
    String? toolName,
    required String status,
    @Default(<String, dynamic>{}) Map<String, dynamic> result,
  }) = _WsAssistantToolResultEvent;
}

@freezed
abstract class WsAssistantConfirmRequestEvent
    with _$WsAssistantConfirmRequestEvent {
  const factory WsAssistantConfirmRequestEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String sessionId,
    required String toolCallId,
    required String toolName,
    @Default(<String, dynamic>{}) Map<String, dynamic> arguments,
    String? summary,
  }) = _WsAssistantConfirmRequestEvent;
}

@freezed
abstract class WsAssistantNavigateEvent with _$WsAssistantNavigateEvent {
  const factory WsAssistantNavigateEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String route,
  }) = _WsAssistantNavigateEvent;
}

/// Live-карточка задачи для Tasks-tab правой панели. Эмитится бэкендом
/// параллельно с project-scoped `task_status` при смене state (см.
/// backend/internal/ws/hubbridge.go fanOutAssistantTaskUpdate).
@freezed
abstract class WsAssistantTaskUpdateEvent with _$WsAssistantTaskUpdateEvent {
  const factory WsAssistantTaskUpdateEvent({
    required DateTime ts,
    required int v,
    required String userId,
    required String projectId,
    required String taskId,
    required String state,
    String? title,
    required DateTime updatedAt,
  }) = _WsAssistantTaskUpdateEvent;
}

enum WsIntegrationStatus {
  connected('connected'),
  disconnected('disconnected'),
  error('error'),
  pending('pending');

  const WsIntegrationStatus(this.jsonValue);
  final String jsonValue;

  static WsIntegrationStatus? tryParse(String raw) {
    for (final v in WsIntegrationStatus.values) {
      if (v.jsonValue == raw) {
        return v;
      }
    }
    return null;
  }
}

@freezed
abstract class WsServerEvent with _$WsServerEvent {
  const factory WsServerEvent.taskStatus(WsTaskStatusEvent value) =
      WsServerEventTaskStatus;
  const factory WsServerEvent.taskMessage(WsTaskMessageEvent value) =
      WsServerEventTaskMessage;
  const factory WsServerEvent.agentLog(WsAgentLogEvent value) =
      WsServerEventAgentLog;
  const factory WsServerEvent.error(WsErrorEvent value) = WsServerEventError;
  const factory WsServerEvent.integrationStatus(
    WsIntegrationStatusEvent value,
  ) = WsServerEventIntegrationStatus;
  const factory WsServerEvent.assistantSessionUpdated(
    WsAssistantSessionUpdatedEvent value,
  ) = WsServerEventAssistantSessionUpdated;
  const factory WsServerEvent.assistantMessage(
    WsAssistantMessageEvent value,
  ) = WsServerEventAssistantMessage;
  const factory WsServerEvent.assistantToolCall(
    WsAssistantToolCallEvent value,
  ) = WsServerEventAssistantToolCall;
  const factory WsServerEvent.assistantToolResult(
    WsAssistantToolResultEvent value,
  ) = WsServerEventAssistantToolResult;
  const factory WsServerEvent.assistantConfirmRequest(
    WsAssistantConfirmRequestEvent value,
  ) = WsServerEventAssistantConfirmRequest;
  const factory WsServerEvent.assistantNavigate(
    WsAssistantNavigateEvent value,
  ) = WsServerEventAssistantNavigate;
  const factory WsServerEvent.assistantTaskUpdate(
    WsAssistantTaskUpdateEvent value,
  ) = WsServerEventAssistantTaskUpdate;
  const factory WsServerEvent.unknown(WsUnknownEvent value) =
      WsServerEventUnknown;
}

// ---------------------------------------------------------------------------
// Единый клиентский стрим (11.9): события сервера, парсинг, сбои сервиса
// ---------------------------------------------------------------------------

@freezed
abstract class WsClientEvent with _$WsClientEvent {
  const factory WsClientEvent.server(WsServerEvent event) = WsClientEventServer;
  const factory WsClientEvent.parseError(WsParseError error) =
      WsClientEventParseError;
  const factory WsClientEvent.serviceFailure(WsServiceFailure failure) =
      WsClientEventServiceFailure;
  const factory WsClientEvent.authFailure(WsAuthFailure failure) =
      WsClientEventAuthFailure;
  const factory WsClientEvent.subprotocolMismatch(WsSubprotocolMismatch info) =
      WsClientEventSubprotocolMismatch;
}

// ---------------------------------------------------------------------------
// URL + envelope parsing
// ---------------------------------------------------------------------------

const int kWsMaxIncomingFrameUtf8Bytes = 256 * 1024;

final RegExp _wsTsOffsetColon = RegExp(r'[+-]\d{2}:\d{2}$');
final RegExp _wsTsOffsetCompact = RegExp(r'[+-]\d{2}\d{2}$');

/// Нормализация [baseUrl] из Dio → `ws`/`wss` + суффикс `/projects/{id}/ws`.
Uri buildProjectWebSocketUri(String baseUrl, String projectId) {
  var trimmed = baseUrl.trim();
  if (trimmed.endsWith('/')) {
    trimmed = trimmed.substring(0, trimmed.length - 1);
  }
  final u = Uri.parse(trimmed);
  final scheme = u.scheme == 'https'
      ? 'wss'
      : u.scheme == 'http'
          ? 'ws'
          : throw ArgumentError.value(
              baseUrl,
              'baseUrl',
              'Ожидался http или https',
            );
  return u.replace(
    scheme: scheme,
    path: '${u.path}/projects/$projectId/ws',
    query: u.query.isEmpty ? null : u.query,
  );
}

/// Парсинг `ts`: только UTC с суффиксом `Z` или numeric offset (`+00:00` и т.д.).
DateTime parseWsTimestamp(String raw, {String? context}) {
  if (raw.isEmpty) {
    throw FormatException('Пустая строка ts', context);
  }
  final parsed = DateTime.tryParse(raw);
  if (parsed == null) {
    throw FormatException('Некорректный RFC3339 ts: $raw', context);
  }
  if (parsed.isUtc) {
    return parsed;
  }
  final hasExplicitOffset =
      _wsTsOffsetColon.hasMatch(raw) || _wsTsOffsetCompact.hasMatch(raw);
  if (hasExplicitOffset) {
    return parsed.toUtc();
  }
  if (raw.endsWith('Z') || raw.endsWith('z')) {
    return parsed.toUtc();
  }
  throw FormatException(
    'ts без Z/explicit offset — запрещено (локальное время): $raw',
    context,
  );
}

/// Список type-дискриминаторов, которые приходят без `project_id` — envelope
/// несёт `user_id`. Sprint 21 §7: assistant.* — все user-scoped.
bool _isUserScopedType(String type) {
  return type == 'integration_status' || type.startsWith('assistant.');
}

/// Парсинг user-scoped envelope (общий вход для integration_status и assistant.*).
WsServerEvent _parseUserScopedEnvelope({
  required String type,
  required DateTime ts,
  required int v,
  required Map<String, dynamic> root,
}) {
  final uid = root['user_id'];
  if (uid is! String) {
    throw WsParseError(
      message: 'user_id отсутствует или не string ($type)',
    );
  }
  final data = root['data'];
  if (data is! Map<String, dynamic>) {
    throw const WsParseError(message: 'data отсутствует или не object');
  }
  final d = data;

  DateTime? optTs(String key, {String? context}) {
    final raw = d[key];
    if (raw is! String || raw.isEmpty) {
      return null;
    }
    try {
      return parseWsTimestamp(raw, context: context ?? '$type.$key');
    } on FormatException {
      return null;
    }
  }

  DateTime requiredTs(String key) {
    final raw = d[key];
    if (raw is! String || raw.isEmpty) {
      throw WsParseError(message: '$key отсутствует или не string ($type)');
    }
    try {
      return parseWsTimestamp(raw, context: '$type.$key');
    } on FormatException catch (e) {
      throw WsParseError(message: e.message, detail: e.source?.toString());
    }
  }

  Map<String, dynamic> asMap(String key) {
    final raw = d[key];
    if (raw is Map<String, dynamic>) {
      return Map<String, dynamic>.from(raw);
    }
    return <String, dynamic>{};
  }

  switch (type) {
    case 'integration_status':
      final providerRaw = d['provider'];
      if (providerRaw is! String || providerRaw.isEmpty) {
        throw const WsParseError(
          message: 'provider отсутствует или пуст (integration_status)',
        );
      }
      final statusRaw = d['status'];
      if (statusRaw is! String) {
        throw const WsParseError(
          message: 'status отсутствует или не string (integration_status)',
        );
      }
      final status = WsIntegrationStatus.tryParse(statusRaw);
      if (status == null) {
        return WsServerEvent.unknown(
          WsUnknownEvent(type: type, ts: ts, v: v, projectId: '', data: d),
        );
      }
      return WsServerEvent.integrationStatus(
        WsIntegrationStatusEvent(
          ts: ts,
          v: v,
          userId: uid,
          provider: providerRaw,
          status: status,
          reason: d['reason'] as String?,
          connectedAt: optTs('connected_at'),
          expiresAt: optTs('expires_at'),
        ),
      );

    case 'assistant.session_updated':
      final sid = d['session_id'];
      if (sid is! String || sid.isEmpty) {
        throw const WsParseError(
          message: 'session_id отсутствует (assistant.session_updated)',
        );
      }
      return WsServerEvent.assistantSessionUpdated(
        WsAssistantSessionUpdatedEvent(
          ts: ts,
          v: v,
          userId: uid,
          sessionId: sid,
          title: d['title'] as String?,
          status: d['status'] as String? ?? '',
          busy: d['busy'] as bool? ?? false,
          lastMessageAt: optTs('last_message_at'),
          updatedAt: requiredTs('updated_at'),
        ),
      );

    case 'assistant.message':
      final sid = d['session_id'];
      final mid = d['message_id'];
      if (sid is! String || sid.isEmpty || mid is! String || mid.isEmpty) {
        throw const WsParseError(
          message: 'session_id/message_id отсутствуют (assistant.message)',
        );
      }
      return WsServerEvent.assistantMessage(
        WsAssistantMessageEvent(
          ts: ts,
          v: v,
          userId: uid,
          sessionId: sid,
          messageId: mid,
          role: d['role'] as String? ?? '',
          content: d['content'] as String?,
          toolCallId: d['tool_call_id'] as String?,
          toolName: d['tool_name'] as String?,
          createdAt: requiredTs('created_at'),
        ),
      );

    case 'assistant.tool_call':
      final sid = d['session_id'];
      final tcid = d['tool_call_id'];
      if (sid is! String || sid.isEmpty || tcid is! String || tcid.isEmpty) {
        throw const WsParseError(
          message: 'session_id/tool_call_id отсутствуют (assistant.tool_call)',
        );
      }
      return WsServerEvent.assistantToolCall(
        WsAssistantToolCallEvent(
          ts: ts,
          v: v,
          userId: uid,
          sessionId: sid,
          messageId: d['message_id'] as String?,
          toolCallId: tcid,
          toolName: d['tool_name'] as String? ?? '',
          arguments: asMap('arguments'),
        ),
      );

    case 'assistant.tool_result':
      final sid = d['session_id'];
      final tcid = d['tool_call_id'];
      if (sid is! String || sid.isEmpty || tcid is! String || tcid.isEmpty) {
        throw const WsParseError(
          message:
              'session_id/tool_call_id отсутствуют (assistant.tool_result)',
        );
      }
      return WsServerEvent.assistantToolResult(
        WsAssistantToolResultEvent(
          ts: ts,
          v: v,
          userId: uid,
          sessionId: sid,
          messageId: d['message_id'] as String?,
          toolCallId: tcid,
          toolName: d['tool_name'] as String?,
          status: d['status'] as String? ?? '',
          result: asMap('result'),
        ),
      );

    case 'assistant.confirm_request':
      final sid = d['session_id'];
      final tcid = d['tool_call_id'];
      if (sid is! String || sid.isEmpty || tcid is! String || tcid.isEmpty) {
        throw const WsParseError(
          message:
              'session_id/tool_call_id отсутствуют (assistant.confirm_request)',
        );
      }
      return WsServerEvent.assistantConfirmRequest(
        WsAssistantConfirmRequestEvent(
          ts: ts,
          v: v,
          userId: uid,
          sessionId: sid,
          toolCallId: tcid,
          toolName: d['tool_name'] as String? ?? '',
          arguments: asMap('arguments'),
          summary: d['summary'] as String?,
        ),
      );

    case 'assistant.navigate':
      final route = d['route'];
      if (route is! String || route.isEmpty) {
        throw const WsParseError(
          message: 'route отсутствует (assistant.navigate)',
        );
      }
      return WsServerEvent.assistantNavigate(
        WsAssistantNavigateEvent(ts: ts, v: v, userId: uid, route: route),
      );

    case 'assistant.task_update':
      final pid = d['project_id'];
      final tid = d['task_id'];
      if (pid is! String || pid.isEmpty || tid is! String || tid.isEmpty) {
        throw const WsParseError(
          message: 'project_id/task_id отсутствуют (assistant.task_update)',
        );
      }
      return WsServerEvent.assistantTaskUpdate(
        WsAssistantTaskUpdateEvent(
          ts: ts,
          v: v,
          userId: uid,
          projectId: pid,
          taskId: tid,
          state: d['state'] as String? ?? '',
          title: d['title'] as String?,
          updatedAt: requiredTs('updated_at'),
        ),
      );

    default:
      return WsServerEvent.unknown(
        WsUnknownEvent(type: type, ts: ts, v: v, projectId: '', data: d),
      );
  }
}

/// Декодирование одного текстового кадра → [WsServerEvent] или бросок [FormatException]/[WsParseError].
WsServerEvent parseWsServerEnvelope(String text) {
  var t = text;
  if (t.startsWith('\uFEFF')) {
    t = t.substring(1);
  }
  final utf8Bytes = utf8.encode(t);
  if (utf8Bytes.length > kWsMaxIncomingFrameUtf8Bytes) {
    throw WsParseError(
      message: 'Превышен лимит UTF-8 кадра',
      detail: '${utf8Bytes.length} > $kWsMaxIncomingFrameUtf8Bytes',
    );
  }

  final dynamic decoded;
  try {
    decoded = jsonDecode(t);
  } catch (e) {
    throw WsParseError(message: 'jsonDecode', detail: '$e');
  }
  if (decoded is! Map<String, dynamic>) {
    throw const WsParseError(message: 'Корень JSON не object');
  }
  final m = decoded;
  final type = m['type'];
  if (type is! String) {
    throw const WsParseError(message: 'Поле type отсутствует или не string');
  }
  final v = m['v'];
  if (v is! int) {
    throw const WsParseError(message: 'Поле v отсутствует или не int');
  }
  final tsRaw = m['ts'];
  if (tsRaw is! String) {
    throw const WsParseError(message: 'Поле ts отсутствует или не string');
  }
  final DateTime ts;
  try {
    ts = parseWsTimestamp(tsRaw, context: 'envelope.ts');
  } on FormatException catch (e) {
    throw WsParseError(message: e.message, detail: e.source?.toString());
  }
  // User-scoped события (см. dashboard-redesign §4a.4 и Sprint 21 §7) приходят
  // с user_id вместо project_id. Для них parsing-ветка ниже отдельная;
  // для project-scoped event'ов сохраняем строгую проверку.
  if (_isUserScopedType(type)) {
    return _parseUserScopedEnvelope(type: type, ts: ts, v: v, root: m);
  }

  final pid = m['project_id'];
  if (pid is! String) {
    throw const WsParseError(message: 'project_id отсутствует или не string');
  }
  final data = m['data'];
  if (data is! Map<String, dynamic>) {
    throw const WsParseError(message: 'data отсутствует или не object');
  }
  final d = data;

  switch (type) {
    case 'task_status':
      return WsServerEvent.taskStatus(
        WsTaskStatusEvent(
          ts: ts,
          v: v,
          projectId: pid,
          taskId: d['task_id'] as String? ?? '',
          parentTaskId: d['parent_task_id'] as String?,
          previousStatus: d['previous_status'] as String? ?? '',
          status: d['status'] as String? ?? '',
          assignedAgentId: d['assigned_agent_id'] as String?,
          agentRole: d['agent_role'] as String?,
          errorMessage: d['error_message'] as String?,
        ),
      );
    case 'task_message':
      return WsServerEvent.taskMessage(
        WsTaskMessageEvent(
          ts: ts,
          v: v,
          projectId: pid,
          taskId: d['task_id'] as String? ?? '',
          messageId: d['message_id'] as String? ?? '',
          senderType: d['sender_type'] as String? ?? '',
          senderId: d['sender_id'] as String? ?? '',
          senderRole: d['sender_role'] as String?,
          messageType: d['message_type'] as String? ?? '',
          content: d['content'] as String? ?? '',
          metadata: (d['metadata'] is Map<String, dynamic>)
              ? Map<String, dynamic>.from(d['metadata']! as Map)
              : <String, dynamic>{},
        ),
      );
    case 'agent_log':
      return WsServerEvent.agentLog(
        WsAgentLogEvent(
          ts: ts,
          v: v,
          projectId: pid,
          taskId: d['task_id'] as String? ?? '',
          sandboxId: d['sandbox_id'] as String? ?? '',
          stream: d['stream'] as String? ?? '',
          line: d['line'] as String? ?? '',
          seq: (d['seq'] is int)
              ? d['seq'] as int
              : int.tryParse('${d['seq']}') ?? 0,
          truncated: d['truncated'] as bool? ?? false,
        ),
      );
    case 'error':
      final codeRaw = d['code'] as String?;
      final code = codeRaw != null ? WsErrorCode.tryParse(codeRaw) : null;
      if (code == null) {
        return WsServerEvent.unknown(
          WsUnknownEvent(
            type: type,
            ts: ts,
            v: v,
            projectId: pid,
            data: d,
          ),
        );
      }
      return WsServerEvent.error(
        WsErrorEvent(
          ts: ts,
          v: v,
          projectId: pid,
          code: code,
          message: d['message'] as String? ?? '',
          details: (d['details'] is Map<String, dynamic>)
              ? Map<String, dynamic>.from(d['details']! as Map)
              : <String, dynamic>{},
          needsRestRefetch: code == WsErrorCode.streamOverflow,
        ),
      );
    default:
      return WsServerEvent.unknown(
        WsUnknownEvent(type: type, ts: ts, v: v, projectId: pid, data: d),
      );
  }
}
