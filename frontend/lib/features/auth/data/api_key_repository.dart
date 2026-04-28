import 'package:dio/dio.dart';
import 'package:frontend/features/auth/domain/auth_exceptions.dart';
import 'package:frontend/features/auth/domain/models.dart';

/// ApiKeyRepository отвечает за работу с API API-ключей
class ApiKeyRepository {
  final Dio _dio;

  ApiKeyRepository({required Dio dio}) : _dio = dio;

  /// Получение списка API-ключей текущего пользователя
  Future<List<ApiKeyModel>> listKeys() async {
    try {
      final response = await _dio.get('/auth/api-keys');
      final data = response.data as List<dynamic>;
      return data
          .map((json) => ApiKeyModel.fromJson(json as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Создание нового API-ключа
  Future<ApiKeyCreatedModel> createKey({
    required String name,
    String? scopes,
    int? expiresInSeconds,
  }) async {
    try {
      final data = <String, dynamic>{'name': name};
      if (scopes != null && scopes.isNotEmpty) {
        data['scopes'] = scopes;
      }
      if (expiresInSeconds != null) {
        data['expires_in'] = expiresInSeconds;
      }

      final response = await _dio.post('/auth/api-keys', data: data);
      return ApiKeyCreatedModel.fromJson(
        response.data as Map<String, dynamic>,
      );
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Отзыв API-ключа
  Future<void> revokeKey(String id) async {
    try {
      await _dio.post('/auth/api-keys/$id/revoke');
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Удаление API-ключа
  Future<void> deleteKey(String id) async {
    try {
      await _dio.delete('/auth/api-keys/$id');
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Получение готовой MCP-конфигурации
  Future<MCPConfigModel> getMCPConfig({String? apiKey}) async {
    try {
      final queryParams = <String, dynamic>{};
      if (apiKey != null) {
        queryParams['apiKey'] = apiKey;
      }

      final response = await _dio.get(
        '/auth/api-keys/mcp-config',
        queryParameters: queryParams,
      );
      return MCPConfigModel.fromJson(response.data as Map<String, dynamic>);
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Обработка ошибок Dio
  AuthException _handleError(DioException error) {
    if (error.response != null) {
      final data = error.response!.data;
      String? errorMessage;

      if (data is Map<String, dynamic>) {
        errorMessage = data['message'] as String?;
      }

      final message = errorMessage ?? error.message;
      final statusCode = error.response!.statusCode;

      switch (statusCode) {
        case 401:
          return const AccessDeniedException('Session expired or invalid');
        case 403:
          return AccessDeniedException(message);
        case 404:
          return UnknownAuthException(message);
        default:
          return UnknownAuthException(message);
      }
    }
    return NetworkException(error.message);
  }
}
