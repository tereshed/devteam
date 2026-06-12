import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/enhancer/domain/enhancer_exceptions.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_change_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_config_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_run_model.dart';

/// HTTP-слой энхансера проекта (`/projects/:id/enhancer*`).
class EnhancerRepository {
  EnhancerRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw EnhancerApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Конфиг энхансера проекта (дефолт, если ещё не настраивался).
  Future<EnhancerConfigModel> getConfig(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/enhancer',
        cancelToken: cancelToken,
      );
      return EnhancerConfigModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Частично обновляет конфиг (создаёт при первом вызове).
  Future<EnhancerConfigModel> updateConfig(
    String projectId, {
    bool? isActive,
    String? cronExpression,
    int? analysisWindowDays,
    int? maxChangesPerRun,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.put(
        '/projects/$projectId/enhancer',
        data: {
          if (isActive != null) 'is_active': isActive,
          if (cronExpression != null) 'cron_expression': cronExpression,
          if (analysisWindowDays != null)
            'analysis_window_days': analysisWindowDays,
          if (maxChangesPerRun != null)
            'max_changes_per_run': maxChangesPerRun,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return EnhancerConfigModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Запускает прогон немедленно. 409 → [EnhancerRunInProgressException].
  Future<EnhancerRunModel> runNow(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.post(
        '/projects/$projectId/enhancer/run',
        cancelToken: cancelToken,
      );
      return EnhancerRunModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Последние прогоны проекта (новые сверху).
  Future<List<EnhancerRunModel>> listRuns(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/enhancer/runs',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = (body['runs'] as List<dynamic>? ?? <dynamic>[]);
      return list
          .map((e) => EnhancerRunModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Применяет предложение. 409 → конфликт (цель изменилась), 422 → bad state.
  Future<EnhancerChangeModel> applyChange(
    String projectId,
    String changeId, {
    CancelToken? cancelToken,
  }) =>
      _changeAction(projectId, changeId, 'apply', cancelToken: cancelToken);

  /// Отклоняет предложение.
  Future<EnhancerChangeModel> rejectChange(
    String projectId,
    String changeId, {
    CancelToken? cancelToken,
  }) =>
      _changeAction(projectId, changeId, 'reject', cancelToken: cancelToken);

  /// Откатывает применённое предложение.
  Future<EnhancerChangeModel> rollbackChange(
    String projectId,
    String changeId, {
    CancelToken? cancelToken,
  }) =>
      _changeAction(projectId, changeId, 'rollback', cancelToken: cancelToken);

  Future<EnhancerChangeModel> _changeAction(
    String projectId,
    String changeId,
    String action, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || changeId.isEmpty) {
      throw ArgumentError('projectId and changeId are required');
    }
    try {
      final response = await _dio.post(
        '/projects/$projectId/enhancer/changes/$changeId/$action',
        cancelToken: cancelToken,
      );
      return EnhancerChangeModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Предложения изменений одного прогона.
  Future<List<EnhancerChangeModel>> listRunChanges(
    String projectId,
    String runId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || runId.isEmpty) {
      throw ArgumentError('projectId and runId are required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/enhancer/runs/$runId/changes',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = (body['changes'] as List<dynamic>? ?? <dynamic>[]);
      return list
          .map((e) => EnhancerChangeModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => EnhancerCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => EnhancerApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => EnhancerForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => EnhancerNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: (msg, err, code) => EnhancerConflictException(
        msg,
        originalError: err,
      ),
      on422: (msg, err, code) => EnhancerValidationException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) => EnhancerApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
