import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/assistant_mcp/domain/assistant_mcp_exceptions.dart';
import 'package:frontend/features/assistant_mcp/domain/models/assistant_mcp_server_model.dart';

/// HTTP-слой MCP-серверов ассистента (`/projects/:id/assistant/mcp-servers`).
class AssistantMcpRepository {
  AssistantMcpRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw AssistantMcpApiException(
          msg,
          statusCode: code,
        ),
      );

  Map<String, dynamic> _payload({
    required String name,
    required String transport,
    required String url,
    required Map<String, String> headers,
    required bool requireConfirmation,
    required bool isEnabled,
  }) =>
      {
        'name': name,
        'transport': transport,
        'url': url,
        'headers': headers,
        'require_confirmation': requireConfirmation,
        'is_enabled': isEnabled,
      };

  /// Список MCP-серверов проекта.
  Future<List<AssistantMcpServerModel>> list(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/assistant/mcp-servers',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = body['servers'] as List<dynamic>? ?? <dynamic>[];
      return list
          .map((e) =>
              AssistantMcpServerModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт MCP-сервер.
  Future<AssistantMcpServerModel> create(
    String projectId, {
    required String name,
    required String transport,
    required String url,
    required Map<String, String> headers,
    required bool requireConfirmation,
    required bool isEnabled,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.post(
        '/projects/$projectId/assistant/mcp-servers',
        data: _payload(
          name: name,
          transport: transport,
          url: url,
          headers: headers,
          requireConfirmation: requireConfirmation,
          isEnabled: isEnabled,
        ),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return AssistantMcpServerModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Обновляет MCP-сервер (полная замена полей).
  Future<AssistantMcpServerModel> update(
    String projectId,
    String serverId, {
    required String name,
    required String transport,
    required String url,
    required Map<String, String> headers,
    required bool requireConfirmation,
    required bool isEnabled,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || serverId.isEmpty) {
      throw ArgumentError('projectId and serverId are required');
    }
    try {
      final response = await _dio.put(
        '/projects/$projectId/assistant/mcp-servers/$serverId',
        data: _payload(
          name: name,
          transport: transport,
          url: url,
          headers: headers,
          requireConfirmation: requireConfirmation,
          isEnabled: isEnabled,
        ),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return AssistantMcpServerModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет MCP-сервер по id.
  Future<void> delete(
    String projectId,
    String serverId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || serverId.isEmpty) {
      throw ArgumentError('projectId and serverId are required');
    }
    try {
      await _dio.delete(
        '/projects/$projectId/assistant/mcp-servers/$serverId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => AssistantMcpCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => AssistantMcpApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => AssistantMcpForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => AssistantMcpNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on422: (msg, err, code) => AssistantMcpValidationException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      // Бэкенд отдаёт 400 на невалидные поля (transport/url/name) — показываем
      // как валидацию; прочие HTTP-коды → общий API-error.
      onOtherHttp: (msg, err, code, status) => status == 400
          ? AssistantMcpValidationException(msg, originalError: err)
          : AssistantMcpApiException(
              msg,
              statusCode: status,
              originalError: err,
            ),
    );
  }
}
