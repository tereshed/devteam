import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_message_path.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';

/// Максимальная длина заголовка чата (символы), синхронно с бэкендом.
const int kMaxConversationTitleLength = 255;

/// Максимальная длина текста сообщения (символы), синхронно с бэкендом.
const int kMaxMessageContentLength = 4096;

/// Максимальный размер тела запроса (байты), синхронно с `http.MaxBytesReader` на бэкенде.
const int kMaxRequestBodyBytes = 1024 * 1024;

/// HTTP-слой для чатов и истории сообщений.
class ConversationRepository {
  ConversationRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  static int _normalizeLimit(int limit) {
    if (limit <= 0) {
      return 20;
    }
    if (limit > 100) {
      return 100;
    }
    return limit;
  }

  static int _normalizeOffset(int offset) {
    if (offset < 0) {
      return 0;
    }
    return offset;
  }

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw ConversationApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Список чатов проекта.
  ///
  /// Параметры [limit]/[offset] нормализуются на клиенте; при некорректных значениях сервер может вернуть 400.
  ///
  /// **404:** только [ProjectNotFoundException] (путь под `/projects/.../conversations`).
  Future<ConversationListResponse> listConversations(
    String projectId, {
    int limit = 20,
    int offset = 0,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }

    final nLimit = _normalizeLimit(limit);
    final nOffset = _normalizeOffset(offset);

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/projects/$projectId/conversations',
        queryParameters: <String, dynamic>{
          'limit': nLimit,
          'offset': nOffset,
        },
        cancelToken: cancelToken,
      );
      return ConversationListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт чат в проекте.
  ///
  /// Лимиты: заголовок до [kMaxConversationTitleLength] символов, тело запроса до [kMaxRequestBodyBytes].
  /// Пустой title после trim на сервере даёт 400 (опционально можно не слать запрос, если `title.trim().isEmpty`).
  ///
  /// **404:** [ProjectNotFoundException].
  Future<ConversationModel> createConversation(
    String projectId,
    CreateConversationRequest request, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/projects/$projectId/conversations',
        data: request.toJson(),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ConversationModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Возвращает чат по id.
  ///
  /// **404:** [ConversationNotFoundException]. **401:** по [statusCode], не по тексту `error` в JSON.
  /// **403:** [ConversationForbiddenException].
  Future<ConversationModel> getConversation(
    String conversationId, {
    CancelToken? cancelToken,
  }) async {
    if (conversationId.isEmpty) {
      throw ArgumentError('conversationId is required');
    }

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/conversations/$conversationId',
        cancelToken: cancelToken,
      );
      return ConversationModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет чат (ожидается **204** без тела по контракту API).
  Future<void> deleteConversation(
    String conversationId, {
    CancelToken? cancelToken,
  }) async {
    if (conversationId.isEmpty) {
      throw ArgumentError('conversationId is required');
    }

    try {
      final response = await _dio.delete<void>(
        '/conversations/$conversationId',
        cancelToken: cancelToken,
      );
      if (response.statusCode != 204) {
        throw ConversationApiException(
          'Expected status 204, got ${response.statusCode}',
          statusCode: response.statusCode,
        );
      }
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// История сообщений.
  ///
  /// На бэкенде по умолчанию сортировка **`ORDER BY created_at DESC, id ASC`**: первая страница (`offset = 0`)
  /// — самые новые сообщения. Порядок списка в ответе не менять здесь; переворот под UI — в слое
  /// presentation (например, `ChatController`, задача 11.4).
  ///
  /// **404:** [ConversationNotFoundException].
  Future<MessageListResponse> getMessages(
    String conversationId, {
    int limit = 20,
    int offset = 0,
    CancelToken? cancelToken,
  }) async {
    if (conversationId.isEmpty) {
      throw ArgumentError('conversationId is required');
    }

    final nLimit = _normalizeLimit(limit);
    final nOffset = _normalizeOffset(offset);

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/conversations/$conversationId/messages',
        queryParameters: <String, dynamic>{
          'limit': nLimit,
          'offset': nOffset,
        },
        cancelToken: cancelToken,
      );
      return MessageListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Отправляет сообщение.
  ///
  /// Требуется заголовок идемпотентности [clientMessageId] — валидный UUID (см. [ArgumentError], если нет).
  /// Лимиты: [kMaxMessageContentLength] символов, тело до [kMaxRequestBodyBytes].
  ///
  /// **201** — новое сообщение, **200** — дубликат по idempotency key.
  Future<SendMessageResult> sendMessage(
    String conversationId,
    SendMessageRequest request, {
    required String clientMessageId,
    CancelToken? cancelToken,
  }) async {
    if (conversationId.isEmpty) {
      throw ArgumentError('conversationId is required');
    }
    if (!isValidUuid(clientMessageId)) {
      throw ArgumentError(
        'clientMessageId must be a valid UUID',
      );
    }

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/conversations/$conversationId/messages',
        data: request.toJson(),
        options: Options(
          contentType: 'application/json',
          headers: <String, String>{
            'X-Client-Message-ID': clientMessageId,
          },
        ),
        cancelToken: cancelToken,
      );

      final code = response.statusCode;
      if (code != 200 && code != 201) {
        throw ConversationApiException(
          'Unexpected status code for sendMessage: $code',
          statusCode: code,
        );
      }

      final msg = ConversationMessageModel.fromJson(_jsonBody(response));
      if (code == 200) {
        return SendMessageResult(message: msg, status: MessageSendStatus.duplicate);
      }
      return SendMessageResult(message: msg, status: MessageSendStatus.created);
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    final p = parseDioApiError(error);
    final statusCode = p.statusCode;

    if (p.isCancellation) {
      return ConversationCancelledException(
        p.sanitizedMessage,
        originalError: error,
      );
    }

    if (statusCode == null) {
      return ConversationApiException(
        p.sanitizedMessage,
        originalError: error,
      );
    }

    switch (statusCode) {
      case 401:
        return UnauthorizedException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 403:
        return ConversationForbiddenException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 404:
        if (isProjectConversationsListPath(p.requestPath)) {
          return ProjectNotFoundException(
            p.sanitizedMessage,
            originalError: error,
            apiErrorCode: p.stableErrorCode,
          );
        }
        return ConversationNotFoundException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 429:
        return ConversationRateLimitedException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      default:
        return ConversationApiException(
          p.sanitizedMessage,
          statusCode: statusCode,
          apiErrorCode: p.stableErrorCode,
          originalError: error,
        );
    }
  }
}
