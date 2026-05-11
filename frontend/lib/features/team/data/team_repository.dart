import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/projects/domain/models.dart';
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

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => TeamCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err) => TeamApiException(
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
