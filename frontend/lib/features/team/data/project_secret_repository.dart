import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

class ProjectSecretRepository {
  ProjectSecretRepository({required Dio dio}) : _dio = dio;
  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => AgentConfigCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) => AgentConfigApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => AgentConfigForbiddenException(msg, originalError: err, apiErrorCode: code),
      on404: (msg, err, code) => AgentConfigNotFoundException(msg, originalError: err, apiErrorCode: code),
      on409: (msg, err, code) => AgentConfigConflictException(msg, originalError: err, apiErrorCode: code),
      onOtherHttp: (msg, err, code, status) => AgentConfigApiException(msg, statusCode: status, originalError: err, apiErrorCode: code),
    );
  }

  Future<List<SecretRefModel>> list(String projectId, {CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.get<List<dynamic>>(
        '/projects/$projectId/secrets',
        cancelToken: cancelToken,
      );
      return resp.data!.map((e) => SecretRefModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<SecretRefModel> set(
    String projectId, {
    required String keyName,
    required String value,
    bool injectAsEnv = false,
    String description = '',
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/projects/$projectId/secrets',
        data: {
          'key_name': keyName,
          'value': value,
          'inject_as_env': injectAsEnv,
          'description': description,
        },
        cancelToken: cancelToken,
      );
      return SecretRefModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> delete(String projectId, String secretId, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete(
        '/projects/$projectId/secrets/$secretId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
