import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/sandbox_services/domain/models/sandbox_service_model.dart';
import 'package:frontend/features/sandbox_services/domain/sandbox_service_exceptions.dart';

/// HTTP-слой деклараций сервис-сайдкаров проекта (`/projects/:id/sandbox-services`).
class SandboxServiceRepository {
  SandboxServiceRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw SandboxServiceApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Список деклараций сервис-сайдкаров проекта.
  Future<List<SandboxServiceModel>> list(
    String projectId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.get(
        '/projects/$projectId/sandbox-services',
        cancelToken: cancelToken,
      );
      final body = _jsonBody(response);
      final list = body['services'] as List<dynamic>? ?? <dynamic>[];
      return list
          .map((e) => SandboxServiceModel.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт/обновляет декларацию (upsert по alias).
  Future<SandboxServiceModel> upsert(
    String projectId, {
    required String alias,
    required bool isEnabled,
    String? kind,
    String? image,
    String? dbName,
    String? dbUser,
    int? port,
    String? seedKind,
    String? seedValue,
    int? readyTimeoutSeconds,
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty) {
      throw ArgumentError('projectId is required');
    }
    try {
      final response = await _dio.put(
        '/projects/$projectId/sandbox-services',
        data: {
          'alias': alias,
          'is_enabled': isEnabled,
          if (kind != null) 'kind': kind,
          if (image != null) 'image': image,
          if (dbName != null) 'db_name': dbName,
          if (dbUser != null) 'db_user': dbUser,
          if (port != null) 'port': port,
          if (seedKind != null) 'seed_kind': seedKind,
          if (seedValue != null) 'seed_value': seedValue,
          if (readyTimeoutSeconds != null)
            'ready_timeout_seconds': readyTimeoutSeconds,
        },
        options: Options(contentType: 'application/json'),
        cancelToken: cancelToken,
      );
      return SandboxServiceModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет декларацию по id.
  Future<void> delete(
    String projectId,
    String serviceId, {
    CancelToken? cancelToken,
  }) async {
    if (projectId.isEmpty || serviceId.isEmpty) {
      throw ArgumentError('projectId and serviceId are required');
    }
    try {
      await _dio.delete(
        '/projects/$projectId/sandbox-services/$serviceId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => SandboxServiceCancelledException(
        msg,
        originalError: err,
      ),
      onMissingStatusCode: (msg, err, _) => SandboxServiceApiException(
        msg,
        originalError: err,
      ),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => SandboxServiceForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => SandboxServiceNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: (msg, err, code) => SandboxServiceApiException(
        msg,
        originalError: err,
      ),
      on422: (msg, err, code) => SandboxServiceValidationException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      onOtherHttp: (msg, err, code, status) => SandboxServiceApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
