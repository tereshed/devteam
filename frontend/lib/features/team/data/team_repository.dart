import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/team/domain/models/team_type_model.dart';
import 'package:frontend/features/team/domain/team_exceptions.dart';

/// HTTP-слой команды проекта (13.1). Мутации — в последующих задачах.
class TeamRepository {
  TeamRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw TeamApiException(
          msg,
          statusCode: code,
        ),
      );

  /// [projectId] — UUID из маршрута `/projects/:id/…`.
  ///
  /// Throws [ArgumentError] если [projectId] пуст.
  /// Throws [TeamProjectMismatchException] если `project_id` в ответе ≠ [projectId].
  /// Throws [TeamNotFoundException] на 404, [TeamForbiddenException] на 403,
  /// [UnauthorizedException] на 401, [TeamCancelledException] при отмене,
  /// [TeamApiException] на прочие ошибки HTTP (GET **409** → [TeamConflictException]).
  Future<TeamModel> getTeam(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }

    try {
      final response = await _dio.get(
        '/projects/$projectId/team',
        cancelToken: cancelToken,
      );
      final team = TeamModel.fromJson(_jsonBody(response));
      if (team.projectId != projectId) {
        throw TeamProjectMismatchException(
          'expected projectId $projectId, got ${team.projectId}',
        );
      }
      return team;
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Получает все команды проекта.
  Future<List<TeamModel>> getTeams(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }

    try {
      final response = await _dio.get(
        '/projects/$projectId/teams',
        cancelToken: cancelToken,
      );
      final list = response.data as List<dynamic>;
      final teams = list.map((e) => TeamModel.fromJson(e as Map<String, dynamic>)).toList();
      for (final team in teams) {
        if (team.projectId != projectId) {
          throw TeamProjectMismatchException(
            'expected projectId $projectId, got ${team.projectId}',
          );
        }
      }
      return teams;
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создает новую команду в проекте.
  Future<TeamModel> createTeam(
    String projectId, {
    required String name,
    required String type,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }

    try {
      final response = await _dio.post(
        '/projects/$projectId/teams',
        data: {
          'name': name,
          'type': type,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      final team = TeamModel.fromJson(_jsonBody(response));
      if (team.projectId != projectId) {
        throw TeamProjectMismatchException(
          'expected projectId $projectId, got ${team.projectId}',
        );
      }
      return team;
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создает нового агента в команде.
  Future<AgentModel> createAgent(
    String projectId,
    String teamId, {
    required String name,
    required String role,
    required String executionKind,
    String? roleDescription,
    String? systemPrompt,
    String? model,
    String? providerKind,
    String? codeBackend,
    double? temperature,
    int? maxTokens,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || teamId.isEmpty) {
      throw ArgumentError('projectId and teamId are required');
    }

    try {
      final response = await _dio.post(
        '/projects/$projectId/teams/$teamId/agents',
        data: {
          'name': name,
          'role': role,
          'execution_kind': executionKind,
          if (roleDescription != null) 'role_description': roleDescription,
          if (systemPrompt != null) 'system_prompt': systemPrompt,
          if (model != null) 'model': model,
          if (providerKind != null) 'provider_kind': providerKind,
          if (codeBackend != null) 'code_backend': codeBackend,
          if (temperature != null) 'temperature': temperature,
          if (maxTokens != null) 'max_tokens': maxTokens,
        },
        cancelToken: cancelToken,
      );
      return AgentModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет команду из проекта.
  Future<void> deleteTeam(
    String projectId,
    String teamId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    if (teamId.isEmpty) {
      throw ArgumentError('teamId is required');
    }

    try {
      await _dio.delete(
        '/projects/$projectId/teams/$teamId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет агента из команды проекта.
  Future<void> deleteAgent(
    String projectId,
    String agentId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    if (agentId.isEmpty) {
      throw ArgumentError('agentId is required');
    }

    try {
      await _dio.delete(
        '/projects/$projectId/team/agents/$agentId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Throws [TeamConflictException] на **409**.
  Future<TeamModel> patchAgent(
    String projectId,
    String agentId,
    Map<String, dynamic> body, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    if (agentId.isEmpty) {
      throw ArgumentError('agentId is required');
    }

    try {
      final response = await _dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: body,
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      final team = TeamModel.fromJson(_jsonBody(response));
      if (team.projectId != projectId) {
        throw TeamProjectMismatchException(
          'expected projectId $projectId, got ${team.projectId}',
        );
      }
      return team;
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Получает доступные типы команд.
  Future<List<TeamTypeModel>> getTeamTypes({
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get(
        '/team-types',
        cancelToken: cancelToken,
      );
      final list = response.data as List<dynamic>;
      return list.map((e) => TeamTypeModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создает новый тип команды (admin-only).
  Future<TeamTypeModel> createTeamType({
    required String code,
    required String name,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/admin/team-types',
        data: {
          'code': code,
          'name': name,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      final data = response.data as Map<String, dynamic>;
      return TeamTypeModel.fromJson(data);
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет тип команды (admin-only).
  Future<void> deleteTeamType(
    String code, {
    CancelToken? cancelToken,
  }) async {
    if (code.isEmpty) {
      throw ArgumentError('code is required');
    }
    try {
      await _dio.delete(
        '/admin/team-types/$code',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => TeamCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => TeamApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => TeamForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => TeamNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: (msg, err, code) => TeamConflictException(
        msg,
        originalError: err,
      ),
      onOtherHttp: (msg, err, code, status) => TeamApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
