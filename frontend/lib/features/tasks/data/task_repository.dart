import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_message_path.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';

/// Лимит пагинации списков задач по умолчанию (синхронно с `normalizeTaskListPagination` на бэкенде).
const int kTaskListDefaultLimit = 50;

/// Верхняя граница `limit` для списков задач (синхронно с бэкендом).
const int kTaskListMaxLimit = 200;

/// Максимальный размер текста коррекции в байтах UTF-8 (см. `UserCorrectionMaxBytes` на бэкенде).
const int kUserCorrectionMaxBytes = 256 * 1024;

/// Сегмент URL для `POST /tasks/:id/{pause|cancel|resume}` (имена совпадают с [Enum.name]).
enum _TaskLifecycleAction {
  pause,
  cancel,
  resume,
}

/// HTTP-слой задач и сообщений задачи.
class TaskRepository {
  TaskRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  static int _normalizeLimit(int limit) {
    if (limit <= 0) {
      return kTaskListDefaultLimit;
    }
    if (limit > kTaskListMaxLimit) {
      return kTaskListMaxLimit;
    }
    return limit;
  }

  static int _normalizeOffset(int offset) {
    if (offset < 0) {
      return 0;
    }
    return offset;
  }

  static void _requireNonEmpty(String value, String name) {
    if (value.isEmpty) {
      throw ArgumentError('$name is required');
    }
  }

  static void _requireUuid(String value, String name) {
    _requireNonEmpty(value, name);
    if (!isValidUuid(value)) {
      throw ArgumentError('$name must be a valid UUID');
    }
  }

  static void _requireOptionalUuid(String? value, String name) {
    if (value == null) {
      return;
    }
    if (value.isEmpty) {
      throw ArgumentError('$name must not be empty');
    }
    if (!isValidUuid(value)) {
      throw ArgumentError('$name must be a valid UUID');
    }
  }

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) {
          throw TaskApiException(
            msg,
            statusCode: code,
            originalError: response,
          );
        },
      );

  /// Список задач проекта.
  ///
  /// На бэкенде при пустых `order_by`/`order_dir` список упорядочен по `created_at DESC` (и нормализация
  /// направления в `sanitizeOrderDir`).
  ///
  /// **404** на `/projects/.../tasks`: только [ProjectNotFoundException].
  Future<TaskListResponse> listTasks(
    String projectId, {
    TaskListFilter? filter,
    int limit = kTaskListDefaultLimit,
    int offset = 0,
    CancelToken? cancelToken,
  }) async {
    _requireUuid(projectId, 'projectId');

    final nLimit = _normalizeLimit(limit);
    final nOffset = _normalizeOffset(offset);

    final query = <String, dynamic>{
      ...?filter?.toQueryParameters(),
      'limit': nLimit,
      'offset': nOffset,
    };

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        queryParameters: query,
        options: Options(listFormat: ListFormat.multi),
        cancelToken: cancelToken,
      );
      return TaskListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт задачу в проекте (**201**).
  ///
  /// **404:** [ProjectNotFoundException].
  Future<TaskModel> createTask(
    String projectId,
    CreateTaskRequest request, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(projectId, 'projectId');

    _requireOptionalUuid(request.parentTaskId, 'parentTaskId');
    _requireOptionalUuid(request.assignedAgentId, 'assignedAgentId');

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        data: request.toJson(),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return TaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Возвращает задачу по id.
  ///
  /// **401:** по [statusCode], не по тексту `error` в JSON. **404:** [TaskNotFoundException].
  Future<TaskModel> getTask(
    String taskId, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/tasks/$taskId',
        cancelToken: cancelToken,
      );
      return TaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Частичное обновление задачи (**200**).
  ///
  /// Если одновременно заданы [UpdateTaskRequest.assignedAgentId] и
  /// [UpdateTaskRequest.clearAssignedAgent] == true, кидает [ArgumentError] до сети.
  Future<TaskModel> updateTask(
    String taskId,
    UpdateTaskRequest request, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    if (request.assignedAgentId != null && request.clearAssignedAgent) {
      throw ArgumentError(
        'assignedAgentId and clearAssignedAgent cannot both be set',
      );
    }

    _requireOptionalUuid(request.assignedAgentId, 'assignedAgentId');

    try {
      final response = await _dio.put<Map<String, dynamic>>(
        '/tasks/$taskId',
        data: request.toJson(),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return TaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет задачу (**204** без тела).
  Future<void> deleteTask(
    String taskId, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    try {
      final response = await _dio.delete<void>(
        '/tasks/$taskId',
        cancelToken: cancelToken,
      );
      if (response.statusCode != 204) {
        throw TaskApiException(
          'Expected status 204, got ${response.statusCode}',
          statusCode: response.statusCode,
        );
      }
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Ставит задачу на паузу (**200**).
  Future<TaskModel> pauseTask(
    String taskId, {
    CancelToken? cancelToken,
  }) =>
      _postTaskAction(taskId, _TaskLifecycleAction.pause, cancelToken: cancelToken);

  /// Отменяет задачу (**200**).
  Future<TaskModel> cancelTask(
    String taskId, {
    CancelToken? cancelToken,
  }) =>
      _postTaskAction(taskId, _TaskLifecycleAction.cancel, cancelToken: cancelToken);

  /// Возобновляет задачу (**200**).
  Future<TaskModel> resumeTask(
    String taskId, {
    CancelToken? cancelToken,
  }) =>
      _postTaskAction(taskId, _TaskLifecycleAction.resume, cancelToken: cancelToken);

  Future<TaskModel> _postTaskAction(
    String taskId,
    _TaskLifecycleAction action, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/tasks/$taskId/${action.name}',
        cancelToken: cancelToken,
      );
      return TaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Коррекция требований к задаче (**200**).
  ///
  /// Бэкенд может модифицировать текст (см. `ValidateAndSanitizeUserCorrection` на сервере): trim,
  /// удаление управляющих символов, вырезание закрывающих тегов `</user_correction>`. UI должен опираться
  /// на данные с сервера после ответа, а не на локально отправленную строку.
  ///
  /// Текст после [String.trim] не должен быть пустым. Размер в UTF-8 не должен превышать
  /// [kUserCorrectionMaxBytes].
  Future<TaskModel> correctTask(
    String taskId,
    CorrectTaskRequest request, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    if (request.text.trim().isEmpty) {
      throw ArgumentError('text must not be empty or whitespace');
    }

    if (utf8.encode(request.text).length > kUserCorrectionMaxBytes) {
      throw ArgumentError('Correction text exceeds maximum size');
    }

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/tasks/$taskId/correct',
        data: request.toJson(),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return TaskModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Сообщения задачи; на бэкенде порядок **`created_at ASC`**.
  Future<TaskMessageListResponse> listTaskMessages(
    String taskId, {
    String? messageType,
    String? senderType,
    int limit = kTaskListDefaultLimit,
    int offset = 0,
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    final nLimit = _normalizeLimit(limit);
    final nOffset = _normalizeOffset(offset);

    final mt = messageType;
    final st = senderType;

    final query = <String, dynamic>{
      'limit': nLimit,
      'offset': nOffset,
      if (mt != null && mt.isNotEmpty) 'message_type': mt,
      if (st != null && st.isNotEmpty) 'sender_type': st,
    };

    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/tasks/$taskId/messages',
        queryParameters: query,
        cancelToken: cancelToken,
      );
      return TaskMessageListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Добавляет сообщение к задаче (**201**).
  Future<TaskMessageModel> addTaskMessage(
    String taskId,
    CreateTaskMessageRequest request, {
    CancelToken? cancelToken,
  }) async {
    _requireUuid(taskId, 'taskId');

    try {
      final response = await _dio.post<Map<String, dynamic>>(
        '/tasks/$taskId/messages',
        data: request.toJson(),
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return TaskMessageModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    final p = parseDioApiError(error);
    final statusCode = p.statusCode;

    if (p.isCancellation) {
      return TaskCancelledException(
        p.sanitizedMessage,
        originalError: error,
      );
    }

    if (statusCode == null) {
      return TaskApiException(
        p.sanitizedMessage,
        originalError: error,
        isNetworkTransportError: p.isNetworkTransportError,
      );
    }

    switch (statusCode) {
      case 401:
        return UnauthorizedException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 403:
        return TaskForbiddenException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 404:
        if (isProjectTasksListPath(p.requestPath)) {
          return ProjectNotFoundException(
            p.sanitizedMessage,
            originalError: error,
            apiErrorCode: p.stableErrorCode,
          );
        }
        if (isTaskResourceApiPath(p.requestPath)) {
          return TaskNotFoundException(
            p.sanitizedMessage,
            originalError: error,
            apiErrorCode: p.stableErrorCode,
          );
        }
        return TaskApiException(
          p.sanitizedMessage,
          statusCode: statusCode,
          apiErrorCode: p.stableErrorCode,
          originalError: error,
        );
      case 409:
        return TaskConflictException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 422:
        return TaskUnprocessableException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      default:
        return TaskApiException(
          p.sanitizedMessage,
          statusCode: statusCode,
          apiErrorCode: p.stableErrorCode,
          originalError: error,
        );
    }
  }
}
