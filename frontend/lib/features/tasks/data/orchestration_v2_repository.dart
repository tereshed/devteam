import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_exceptions.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';

/// Чтение v2-данных задачи: артефакты + router decisions.
///
/// Эндпоинты идут в паре с MCP-инструментами (`artifact_list`,
/// `router_decision_list`) — read-only REST для фронтенда.
class OrchestrationV2Repository {
  OrchestrationV2Repository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          OrchestrationV2CancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          OrchestrationV2ApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => OrchestrationV2ForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => OrchestrationV2NotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: null,
      onOtherHttp: (msg, err, code, status) => OrchestrationV2ApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }

  List<dynamic> _asList(dynamic data) {
    if (data is List) {
      return data;
    }
    if (data is Map<String, dynamic>) {
      final raw = data['items'];
      if (raw is List) {
        return raw;
      }
    }
    return const [];
  }

  Future<List<Artifact>> listArtifacts(String taskId,
      {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get(
        '/tasks/$taskId/artifacts',
        cancelToken: cancelToken,
      );
      return _asList(response.data)
          .whereType<Map<String, dynamic>>()
          .map(Artifact.fromJson)
          .toList(growable: false);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<List<RouterDecision>> listRouterDecisions(String taskId,
      {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get(
        '/tasks/$taskId/router-decisions',
        cancelToken: cancelToken,
      );
      return _asList(response.data)
          .whereType<Map<String, dynamic>>()
          .map(RouterDecision.fromJson)
          .toList(growable: false);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
