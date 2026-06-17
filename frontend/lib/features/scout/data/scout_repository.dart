import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/scout/domain/models/scout_config_model.dart';
import 'package:frontend/features/scout/domain/models/scout_run_model.dart';
import 'package:frontend/features/scout/domain/scout_exceptions.dart';

/// HTTP-слой разведчика проекта (`/projects/:id/scout*`).
class ScoutRepository {
  ScoutRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw ScoutApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Конфиг разведчика проекта (дефолт, если ещё не настраивался).
  Future<ScoutConfigModel> getConfig(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/scout',
        cancelToken: cancelToken,
      );
      return ScoutConfigModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Частично обновляет конфиг (создаёт при первом вызове).
  Future<ScoutConfigModel> updateConfig(
    String projectId, {
    bool? isEnabled,
    String? prompt,
    String? codeBackend,
    String? providerKind,
    double? temperature,
    Map<String, dynamic>? codeBackendSettings,
    Map<String, dynamic>? sandboxPermissions,
    String? subscriptionId,
    int? timeoutSeconds,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.put(
        '/projects/$projectId/scout',
        data: {
          if (isEnabled != null) 'is_enabled': isEnabled,
          if (prompt != null) 'prompt': prompt,
          if (codeBackend != null) 'code_backend': codeBackend,
          if (providerKind != null) 'provider_kind': providerKind,
          if (temperature != null) 'temperature': temperature,
          if (codeBackendSettings != null)
            'code_backend_settings': codeBackendSettings,
          if (sandboxPermissions != null)
            'sandbox_permissions': sandboxPermissions,
          if (subscriptionId != null) 'subscription_id': subscriptionId,
          if (timeoutSeconds != null) 'timeout_seconds': timeoutSeconds,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ScoutConfigModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Запускает прогон разведки по постановке проблемы (асинхронно).
  Future<ScoutRunModel> dispatch(
    String projectId, {
    required String problem,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.post(
        '/projects/$projectId/scout/run',
        data: {'problem': problem},
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return ScoutRunModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Последние прогоны проекта (новые сверху).
  Future<List<ScoutRunModel>> listRuns(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/scout/runs',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = body['runs'] as List<dynamic>? ?? <dynamic>[];
      return list
          .map((e) => ScoutRunModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => ScoutCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => ScoutApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => ScoutForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => ScoutNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: (msg, err, code) => ScoutApiException(
        msg,
        originalError: err,
      ),
      on422: (msg, err, code) => ScoutValidationException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) => ScoutApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
