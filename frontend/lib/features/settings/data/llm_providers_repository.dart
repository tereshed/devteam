import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/settings/domain/llm_providers_exceptions.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';

/// Sprint 15.29 / 15.B (F9, C1) — HTTP-слой LLM-провайдеров.
/// Использует канонический [mapDioExceptionForRepository]: 401→[UnauthorizedException],
/// 403/404/409 → специализированные подклассы [LLMProvidersException].
class LLMProvidersRepository {
  LLMProvidersRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;
  static const _basePath = '/llm-providers';

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          LLMProvidersCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          LLMProvidersApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => LLMProvidersForbiddenException(msg,
          originalError: err, apiErrorCode: code),
      on404: (msg, err, code) => LLMProvidersNotFoundException(msg,
          originalError: err, apiErrorCode: code),
      on409: (msg, err, code) => LLMProvidersConflictException(msg,
          originalError: err, apiErrorCode: code),
      onOtherHttp: (msg, err, code, status) => LLMProvidersApiException(msg,
          statusCode: status, originalError: err, apiErrorCode: code),
    );
  }

  Future<List<LLMProviderModel>> list({
    bool onlyEnabled = false,
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<dynamic>(
        _basePath,
        queryParameters: {if (onlyEnabled) 'only_enabled': 'true'},
        cancelToken: cancelToken,
      );
      final data = resp.data;
      if (data is! List) {
        throw LLMProvidersApiException('Expected array response');
      }
      return data
          .map((e) => LLMProviderModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<LLMProviderModel> create({
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    String? credential,
    String defaultModel = '',
    bool enabled = true,
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        _basePath,
        data: <String, dynamic>{
          'name': name,
          'kind': kind,
          'base_url': baseURL,
          'auth_type': authType,
          if (credential != null) 'credential': credential,
          'default_model': defaultModel,
          'enabled': enabled,
        },
        cancelToken: cancelToken,
      );
      return LLMProviderModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<LLMProviderModel> update({
    required String id,
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    String? credential,
    String defaultModel = '',
    bool enabled = true,
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.put<Map<String, dynamic>>(
        '$_basePath/$id',
        data: <String, dynamic>{
          'name': name,
          'kind': kind,
          'base_url': baseURL,
          'auth_type': authType,
          if (credential != null && credential.isNotEmpty)
            'credential': credential,
          'default_model': defaultModel,
          'enabled': enabled,
        },
        cancelToken: cancelToken,
      );
      return LLMProviderModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> delete(String id, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete<dynamic>('$_basePath/$id', cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> healthCheck(String id, {CancelToken? cancelToken}) async {
    try {
      await _dio.post<dynamic>('$_basePath/$id/health-check',
          cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> testConnection({
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    required String credential,
    String defaultModel = '',
    CancelToken? cancelToken,
  }) async {
    try {
      await _dio.post<dynamic>(
        '$_basePath/test-connection',
        data: <String, dynamic>{
          'name': name,
          'kind': kind,
          'base_url': baseURL,
          'auth_type': authType,
          'credential': credential,
          'default_model': defaultModel,
        },
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
