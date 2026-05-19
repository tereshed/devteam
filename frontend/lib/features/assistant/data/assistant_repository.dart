import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/domain/assistant_active_task_model.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/domain/assistant_status_model.dart';

/// Максимальная длина текста user-сообщения (символы), синхронно с бэкендом
/// (assistant_handler.go `assistantMaxMessageLength = 4096`).
const int kMaxAssistantMessageLength = 4096;

/// Максимальный размер тела запроса (байты), синхронно с `http.MaxBytesReader`.
const int kMaxAssistantRequestBodyBytes = 1024 * 1024;

/// Дефолт/максимум пагинации истории (см. handler `assistantDefaultMessageLimit`).
const int kAssistantDefaultMessageLimit = 30;
const int kAssistantMaxMessageLimit = 100;

/// Дефолт/максимум списка сессий.
const int kAssistantDefaultSessionLimit = 50;
const int kAssistantMaxSessionLimit = 200;

/// HTTP-слой глобального ассистента (Sprint 21 §4 backend).
///
/// Параллель `ConversationRepository`, но scope=user: REST-ручки без projectId,
/// сообщения имеют tool-call поля, идемпотентность через
/// [SendAssistantMessageRequestBuilder.clientMessageId] (UUIDv4).
class AssistantRepository {
  AssistantRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  static int _normalizeMessageLimit(int limit) {
    if (limit <= 0) {
      return kAssistantDefaultMessageLimit;
    }
    if (limit > kAssistantMaxMessageLimit) {
      return kAssistantMaxMessageLimit;
    }
    return limit;
  }

  static int _normalizeSessionLimit(int limit) {
    if (limit <= 0) {
      return kAssistantDefaultSessionLimit;
    }
    if (limit > kAssistantMaxSessionLimit) {
      return kAssistantMaxSessionLimit;
    }
    return limit;
  }

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) =>
            throw AssistantApiException(msg, statusCode: code),
      );

  // ──────────────────────────── Status ────────────────────────────

  /// `GET /assistant/status` — проверить статус конфигурации.
  Future<AssistantStatusModel> getStatus({
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/assistant/status',
        cancelToken: cancelToken,
      );
      return AssistantStatusModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  // ──────────────────────────── Sessions ────────────────────────────

  /// `POST /assistant/sessions` — создать пустую сессию.
  Future<AssistantSessionModel> createSession({CancelToken? cancelToken}) async {
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/assistant/sessions',
        // Бэкенд ожидает пустое тело (CreateAssistantSessionRequest{}), но
        // Gin требует application/json при ShouldBindJSON. Шлём `{}`.
        data: <String, dynamic>{},
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return AssistantSessionModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// `GET /assistant/sessions`.
  Future<AssistantSessionListResponse> listSessions({
    bool includeArchived = false,
    int limit = kAssistantDefaultSessionLimit,
    CancelToken? cancelToken,
  }) async {
    final nLimit = _normalizeSessionLimit(limit);
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/assistant/sessions',
        queryParameters: <String, dynamic>{
          if (includeArchived) 'include_archived': true,
          'limit': nLimit,
        },
        cancelToken: cancelToken,
      );
      return AssistantSessionListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// `GET /assistant/sessions/:id`.
  Future<AssistantSessionModel> getSession(
    String sessionId, {
    CancelToken? cancelToken,
  }) async {
    if (sessionId.isEmpty) {
      throw ArgumentError('sessionId is required');
    }
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/assistant/sessions/$sessionId',
        cancelToken: cancelToken,
      );
      return AssistantSessionModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// `DELETE /assistant/sessions/:id` — soft-archive (204 No Content).
  Future<void> archiveSession(
    String sessionId, {
    CancelToken? cancelToken,
  }) async {
    if (sessionId.isEmpty) {
      throw ArgumentError('sessionId is required');
    }
    try {
      final response = await _dio.delete<void>(
        '/assistant/sessions/$sessionId',
        cancelToken: cancelToken,
      );
      if (response.statusCode != 204) {
        throw AssistantApiException(
          'Expected status 204, got ${response.statusCode}',
          statusCode: response.statusCode,
        );
      }
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  // ──────────────────────────── Messages ────────────────────────────

  /// `GET /assistant/sessions/:id/messages` — курсорная пагинация.
  ///
  /// Первая страница: оба `beforeCreatedAt`/`beforeId` равны `null`.
  /// Следующая страница: брать `nextBeforeCreatedAt` / `nextBeforeId` из
  /// предыдущего ответа. Бэкенд требует «оба или ни одного» (handler
  /// `parseCursorQuery`).
  Future<AssistantMessageListResponse> getMessages(
    String sessionId, {
    int limit = kAssistantDefaultMessageLimit,
    DateTime? beforeCreatedAt,
    String? beforeId,
    CancelToken? cancelToken,
  }) async {
    if (sessionId.isEmpty) {
      throw ArgumentError('sessionId is required');
    }
    if ((beforeCreatedAt == null) != (beforeId == null)) {
      throw ArgumentError(
        'beforeCreatedAt and beforeId must be provided together',
      );
    }
    final nLimit = _normalizeMessageLimit(limit);
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/assistant/sessions/$sessionId/messages',
        queryParameters: <String, dynamic>{
          'limit': nLimit,
          if (beforeId != null) 'before_id': beforeId,
          if (beforeCreatedAt != null)
            'before_created_at': beforeCreatedAt.toUtc().toIso8601String(),
        },
        cancelToken: cancelToken,
      );
      return AssistantMessageListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// `POST /assistant/sessions/:id/messages` — отправить user-сообщение.
  ///
  /// [clientMessageId] обязателен и обязан быть UUIDv4 (см.
  /// [generateClientMessageId]). При повторе с тем же ключом бэкенд вернёт
  /// тот же `message.id` с `duplicate=true` — фронт не показывает typing.
  ///
  /// Статусы: 202 (новое) и 200 (дубликат) — оба валидны.
  Future<SendAssistantMessageResponse> sendMessage(
    String sessionId, {
    required String content,
    required String clientMessageId,
    CancelToken? cancelToken,
  }) async {
    if (sessionId.isEmpty) {
      throw ArgumentError('sessionId is required');
    }
    if (!isValidUuid(clientMessageId)) {
      throw ArgumentError('clientMessageId must be a valid UUID');
    }
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/assistant/sessions/$sessionId/messages',
        data: <String, dynamic>{
          'content': content,
          'client_message_id': clientMessageId,
        },
        options: Options(
          contentType: 'application/json',
          headers: <String, String>{
            'X-Client-Message-ID': clientMessageId,
          },
        ),
        cancelToken: cancelToken,
      );
      final code = response.statusCode;
      if (code != 200 && code != 202) {
        throw AssistantApiException(
          'Unexpected status code for sendMessage: $code',
          statusCode: code,
        );
      }
      return SendAssistantMessageResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// `POST /assistant/sessions/:id/confirm` — Approve/Deny destructive op.
  Future<ConfirmToolCallResponse> confirmToolCall(
    String sessionId, {
    required String toolCallId,
    required bool approved,
    String? clientRequestId,
    CancelToken? cancelToken,
  }) async {
    if (sessionId.isEmpty) {
      throw ArgumentError('sessionId is required');
    }
    if (toolCallId.isEmpty) {
      throw ArgumentError('toolCallId is required');
    }
    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/assistant/sessions/$sessionId/confirm',
        data: <String, dynamic>{
          'tool_call_id': toolCallId,
          'approved': approved,
          if (clientRequestId != null && clientRequestId.isNotEmpty)
            'client_request_id': clientRequestId,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ConfirmToolCallResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  // ──────────────────────────── Active tasks ────────────────────────────

  /// `GET /assistant/active-tasks` — bootstrap для Tasks-tab правой панели.
  Future<AssistantActiveTasksResponse> getActiveTasks({
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/assistant/active-tasks',
        cancelToken: cancelToken,
      );
      return AssistantActiveTasksResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  // ──────────────────────────── Error mapping ────────────────────────────

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => AssistantCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, isTransport) => AssistantApiException(
        msg,
        originalError: err,
        isNetworkTransportError: isTransport,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => AssistantForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => AssistantSessionNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: (msg, err, code) => _map409(msg, err, code),
      on429: (msg, err, code) => AssistantRateLimitedException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) => AssistantApiException(
        msg,
        statusCode: status,
        apiErrorCode: code,
        originalError: err,
      ),
    );
  }

  /// 409 → распознаём по `error` коду из тела (контракт
  /// `respondAssistantError` в `assistant_handler.go`).
  ///
  /// - `session_busy`         → [AssistantSessionBusyException] (+ `pendingToolCallId` из тела)
  /// - `no_pending_confirmation` → [AssistantNoPendingConfirmationException]
  /// - `already_confirmed`    → [AssistantAlreadyConfirmedException]
  /// - иное                    → [AssistantApiException] (status=409)
  Exception _map409(String msg, DioException err, String? apiErrorCode) {
    switch (apiErrorCode) {
      case 'session_busy':
        return AssistantSessionBusyException(
          msg,
          originalError: err,
          pendingToolCallId: _extractPendingToolCallId(err),
        );
      case 'no_pending_confirmation':
        return AssistantNoPendingConfirmationException(
          msg,
          originalError: err,
        );
      case 'already_confirmed':
        return AssistantAlreadyConfirmedException(
          msg,
          originalError: err,
        );
      default:
        return AssistantApiException(
          msg,
          statusCode: 409,
          apiErrorCode: apiErrorCode,
          originalError: err,
        );
    }
  }

  /// Достаёт `pending_tool_call_id` из тела 409-ответа. `apierror.JSON` шлёт
  /// его как поле верхнего уровня в JSON-объекте ошибки.
  String? _extractPendingToolCallId(DioException err) {
    final data = err.response?.data;
    if (data is! Map) return null;
    final raw = data['pending_tool_call_id'];
    if (raw is String && raw.isNotEmpty) return raw;
    return null;
  }
}
