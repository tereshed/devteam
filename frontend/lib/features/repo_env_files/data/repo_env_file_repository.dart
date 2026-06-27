import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/repo_env_files/domain/models/repo_env_file_model.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';

/// REST-клиент «инъекции env-файла» уровня репозитория (один файл на репо).
class RepoEnvFileRepository {
  RepoEnvFileRepository({required Dio dio}) : _dio = dio;
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

  /// Возвращает env-файл репозитория или null, если он не настроен (204 No Content).
  Future<RepoEnvFileModel?> get(
    String projectId,
    String repoId, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/projects/$projectId/repositories/$repoId/env-file',
        cancelToken: cancelToken,
      );
      if (resp.statusCode == 204 || resp.data == null) {
        return null;
      }
      return RepoEnvFileModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<RepoEnvFileModel> set(
    String projectId,
    String repoId, {
    required String fileName,
    required String content,
    String targetDir = '',
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.put<Map<String, dynamic>>(
        '/projects/$projectId/repositories/$repoId/env-file',
        data: {
          'file_name': fileName,
          'target_dir': targetDir,
          'content': content,
        },
        cancelToken: cancelToken,
      );
      return RepoEnvFileModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> delete(
    String projectId,
    String repoId, {
    CancelToken? cancelToken,
  }) async {
    try {
      await _dio.delete(
        '/projects/$projectId/repositories/$repoId/env-file',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
