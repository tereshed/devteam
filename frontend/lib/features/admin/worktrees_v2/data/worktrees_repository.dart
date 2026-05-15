import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktrees_exceptions.dart';

/// HTTP-клиент к v2 worktrees debug API.
class WorktreesRepository {
  WorktreesRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          WorktreesCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          WorktreesApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => WorktreesForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => WorktreesNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: null,
      onOtherHttp: (msg, err, code, status) => WorktreesApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }

  Future<List<WorktreeV2>> list({
    String? taskId,
    String? state,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get(
        '/worktrees',
        queryParameters: {
          if (taskId != null) 'task_id': taskId,
          if (state != null) 'state': state,
        },
        cancelToken: cancelToken,
      );
      final data = response.data;
      // Допустимы оба формата: {"items": [...]} или просто [...]
      final list = data is Map<String, dynamic>
          ? (data['items'] as List<dynamic>? ?? const [])
          : (data is List<dynamic> ? data : const []);
      return list
          .whereType<Map<String, dynamic>>()
          .map(WorktreeV2.fromJson)
          .toList(growable: false);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
