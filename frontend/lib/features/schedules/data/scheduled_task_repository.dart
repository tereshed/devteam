import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/schedules/domain/models/scheduled_task_model.dart';
import 'package:frontend/features/schedules/domain/scheduled_task_exceptions.dart';

/// HTTP-слой регулярных (cron) задач проекта (`/projects/:id/scheduled-tasks`).
class ScheduledTaskRepository {
  ScheduledTaskRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw ScheduledTaskApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Список расписаний проекта.
  Future<List<ScheduledTaskModel>> list(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/scheduled-tasks',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = (body['scheduled_tasks'] as List<dynamic>? ?? <dynamic>[]);
      return list
          .map((e) => ScheduledTaskModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт расписание.
  Future<ScheduledTaskModel> create(
    String projectId, {
    required String name,
    required String cronExpression,
    String description = '',
    String? priority,
    String? teamId,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.post(
        '/projects/$projectId/scheduled-tasks',
        data: {
          'name': name,
          'cron_expression': cronExpression,
          'description': description,
          if (priority != null) 'priority': priority,
          if (teamId != null) 'team_id': teamId,
          if (isActive != null) 'is_active': isActive,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ScheduledTaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Частично обновляет расписание.
  Future<ScheduledTaskModel> update(
    String projectId,
    String scheduleId, {
    String? name,
    String? cronExpression,
    String? description,
    String? priority,
    String? teamId,
    bool clearTeam = false,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || scheduleId.isEmpty) {
      throw ArgumentError('projectId and scheduleId are required');
    }
    try {
      final response = await _dio.put(
        '/projects/$projectId/scheduled-tasks/$scheduleId',
        data: {
          if (name != null) 'name': name,
          if (cronExpression != null) 'cron_expression': cronExpression,
          if (description != null) 'description': description,
          if (priority != null) 'priority': priority,
          if (teamId != null) 'team_id': teamId,
          if (clearTeam) 'clear_team': true,
          if (isActive != null) 'is_active': isActive,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ScheduledTaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет расписание.
  Future<void> delete(
    String projectId,
    String scheduleId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || scheduleId.isEmpty) {
      throw ArgumentError('projectId and scheduleId are required');
    }
    try {
      await _dio.delete(
        '/projects/$projectId/scheduled-tasks/$scheduleId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => ScheduledTaskCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => ScheduledTaskApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => ScheduledTaskForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => ScheduledTaskNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on422: (msg, err, code) => ScheduledTaskValidationException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) => ScheduledTaskApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
