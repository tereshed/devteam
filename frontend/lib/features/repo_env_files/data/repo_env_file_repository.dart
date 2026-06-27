import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/repo_env_files/domain/models/repo_env_file_model.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';

/// REST-клиент «инъекции env-файлов» уровня репозитория (несколько файлов на репо).
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

  String _base(String projectId, String repoId) =>
      '/projects/$projectId/repositories/$repoId/env-files';

  Future<List<RepoEnvFileModel>> list(
    String projectId,
    String repoId, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<List<dynamic>>(
        _base(projectId, repoId),
        cancelToken: cancelToken,
      );
      return (resp.data ?? [])
          .map((e) => RepoEnvFileModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<RepoEnvFileModel> create(
    String projectId,
    String repoId, {
    required String fileName,
    required String content,
    String targetDir = '',
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        _base(projectId, repoId),
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

  Future<RepoEnvFileModel> update(
    String projectId,
    String repoId,
    String fileId, {
    required String fileName,
    required String content,
    String targetDir = '',
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.put<Map<String, dynamic>>(
        '${_base(projectId, repoId)}/$fileId',
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
    String repoId,
    String fileId, {
    CancelToken? cancelToken,
  }) async {
    try {
      await _dio.delete(
        '${_base(projectId, repoId)}/$fileId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
