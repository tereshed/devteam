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
      on409: (msg, err, code) => WorktreesConflictException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) {
        // 503 + apiErrorCode == "feature_not_configured" — backend без
        // WORKTREES_ROOT/REPO_ROOT (legacy clone-path). UI показывает
        // конкретное сообщение, а не generic "что-то сломалось".
        // Любой другой 503 (например прокси/балансер свалились) → generic
        // WorktreesApiException, чтобы оператор увидел raw status и пошёл
        // в логи backend'а.
        if (status == 503 && code == 'feature_not_configured') {
          return WorktreesNotConfiguredException(
            msg,
            originalError: err,
            apiErrorCode: code,
          );
        }
        return WorktreesApiException(
          msg,
          statusCode: status,
          originalError: err,
        );
      },
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

  /// Sprint 17 / 6.3 — manual unstick. Вызывает POST /worktrees/{id}/release.
  ///
  /// Backend admin-only; маппинг ошибок:
  ///   - 404 → [WorktreesNotFoundException]
  ///   - 409 → [WorktreesConflictException] (worktree уже released — info-toast)
  ///   - 403 → [WorktreesForbiddenException] (не admin)
  ///
  /// Возвращает обновлённую модель worktree (state='released'). Caller должен
  /// инвалидировать список, чтобы не отображать stale-стейт соседних строк.
  Future<WorktreeV2> release(String id, {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.post(
        '/worktrees/$id/release',
        cancelToken: cancelToken,
      );
      final data = response.data;
      if (data is Map<String, dynamic>) {
        return WorktreeV2.fromJson(data);
      }
      throw WorktreesApiException(
        'Unexpected release response shape: ${data.runtimeType}',
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
