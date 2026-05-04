import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';

/// ProjectRepository отвечает за работу с API проектов
///
/// Абстрагирует логику получения данных от backend API от бизнес-логики.
/// Используется в контроллерах/провайдерах (Riverpod) для получения данных.
///
/// При отмене запроса ([CancelToken.cancel], [DioExceptionType.cancel]) методы бросают
/// [ProjectCancelledException].
class ProjectRepository {
  final Dio _dio;

  ProjectRepository({required Dio dio}) : _dio = dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw ProjectApiException(
          msg,
          statusCode: code,
        ),
      );

  /// Получает проект по UUID
  ///
  /// Throws [ProjectNotFoundException] если проект не найден (404)
  /// Throws [ProjectForbiddenException] если нет прав на доступ (403)
  /// Throws [UnauthorizedException] если не авторизован (401)
  /// Throws [ProjectCancelledException] при отмене запроса
  /// Throws [ProjectApiException] при других ошибках API
  Future<ProjectModel> getProject(
    String id, {
    CancelToken? cancelToken,
  }) async {
    if (id.isEmpty) {
      throw ArgumentError('id is required');
    }

    try {
      final response = await _dio.get(
        '/projects/$id',
        cancelToken: cancelToken,
      );
      return ProjectModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Получает список проектов с фильтрацией и пагинацией
  ///
  /// Нормализует параметры пагинации:
  /// - limit ≤ 0 → 20
  /// - limit > 100 → 100
  /// - offset < 0 → 0
  ///
  /// Throws [UnauthorizedException] если не авторизован (401)
  /// Throws [ProjectCancelledException] при отмене запроса
  /// Throws [ProjectApiException] при других ошибках API
  Future<ProjectListResponse> listProjects({
    ProjectListFilter? filter,
    int limit = 20,
    int offset = 0,
    CancelToken? cancelToken,
  }) async {
    // Нормализация параметров
    var normalizedLimit = limit;
    var normalizedOffset = offset;

    if (normalizedLimit <= 0) {
      normalizedLimit = 20;
    }
    if (normalizedLimit > 100) {
      normalizedLimit = 100;
    }
    if (normalizedOffset < 0) {
      normalizedOffset = 0;
    }

    try {
      final queryParams = {
        'limit': normalizedLimit,
        'offset': normalizedOffset,
        ...?filter?.toQueryParameters(),
      };

      final response = await _dio.get(
        '/projects',
        queryParameters: queryParams,
        cancelToken: cancelToken,
      );

      return ProjectListResponse.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт новый проект
  ///
  /// Throws [ProjectForbiddenException] если нет прав на создание (403)
  /// Throws [ProjectConflictException] если имя занято (409)
  /// Throws [UnauthorizedException] если не авторизован (401)
  /// Throws [ProjectCancelledException] при отмене запроса
  /// Throws [ProjectApiException] при других ошибках API
  Future<ProjectModel> createProject(
    CreateProjectRequest request, {
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/projects',
        data: request.toJson(),
        cancelToken: cancelToken,
      );
      return ProjectModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Обновляет существующий проект (partial update)
  ///
  /// Throws [ArgumentError] если id пуст
  /// Throws [ArgumentError] если одновременно установлены gitCredentialId и removeGitCredential
  /// Throws [ProjectNotFoundException] если проект не найден (404)
  /// Throws [ProjectForbiddenException] если нет прав на обновление (403)
  /// Throws [ProjectConflictException] если имя занято (409)
  /// Throws [UnauthorizedException] если не авторизован (401)
  /// Throws [ProjectCancelledException] при отмене запроса
  /// Throws [ProjectApiException] при других ошибках API
  Future<ProjectModel> updateProject(
    String id,
    UpdateProjectRequest request, {
    CancelToken? cancelToken,
  }) async {
    if (id.isEmpty) {
      throw ArgumentError('id is required');
    }

    if (request.gitCredentialId != null && request.removeGitCredential) {
      throw ArgumentError('cannot set and remove credential simultaneously');
    }

    try {
      final response = await _dio.put(
        '/projects/$id',
        data: request.toJson(),
        cancelToken: cancelToken,
      );
      return ProjectModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Удаляет проект по UUID
  ///
  /// Throws [ArgumentError] если id пуст
  /// Throws [ProjectNotFoundException] если проект не найден (404)
  /// Throws [ProjectForbiddenException] если нет прав на удаление (403)
  /// Throws [UnauthorizedException] если не авторизован (401)
  /// Throws [ProjectCancelledException] при отмене запроса
  /// Throws [ProjectApiException] при других ошибках API
  Future<void> deleteProject(
    String id, {
    CancelToken? cancelToken,
  }) async {
    if (id.isEmpty) {
      throw ArgumentError('id is required');
    }

    try {
      await _dio.delete(
        '/projects/$id',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Обработка ошибок Dio
  Exception _handleDioError(DioException error) {
    final p = parseDioApiError(error);
    final statusCode = p.statusCode;

    if (p.isCancellation) {
      return ProjectCancelledException(
        p.sanitizedMessage,
        originalError: error,
      );
    }

    if (statusCode == null) {
      return ProjectApiException(
        p.sanitizedMessage,
        originalError: error,
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
        return ProjectForbiddenException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 404:
        return ProjectNotFoundException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      case 409:
        return ProjectConflictException(
          p.sanitizedMessage,
          originalError: error,
          apiErrorCode: p.stableErrorCode,
        );
      default:
        return ProjectApiException(
          p.sanitizedMessage,
          statusCode: statusCode,
          originalError: error,
        );
    }
  }
}
