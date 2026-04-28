import 'package:dio/dio.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';

/// ProjectRepository отвечает за работу с API проектов
///
/// Абстрагирует логику получения данных от backend API от бизнес-логики.
/// Используется в контроллерах/провайдерах (Riverpod) для получения данных.
class ProjectRepository {
  final Dio _dio;

  ProjectRepository({required Dio dio}) : _dio = dio;

  /// Получает проект по UUID
  ///
  /// Throws [ProjectNotFoundException] если проект не найден (404)
  /// Throws [ProjectForbiddenException] если нет прав на доступ (403)
  /// Throws [UnauthorizedException] если не авторизован (401)
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
      return ProjectModel.fromJson(response.data as Map<String, dynamic>);
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

    if (normalizedLimit <= 0) normalizedLimit = 20;
    if (normalizedLimit > 100) normalizedLimit = 100;
    if (normalizedOffset < 0) normalizedOffset = 0;

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

      final data = response.data as Map<String, dynamic>;
      final projects = (data['projects'] as List<dynamic>?)
              ?.map((p) => ProjectModel.fromJson(p as Map<String, dynamic>))
              .toList() ??
          [];

      return ProjectListResponse(
        projects: projects,
        total: data['total'] as int? ?? 0,
        limit: data['limit'] as int? ?? normalizedLimit,
        offset: data['offset'] as int? ?? normalizedOffset,
      );
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  /// Создаёт новый проект
  ///
  /// Throws [ProjectForbiddenException] если нет прав на создание (403)
  /// Throws [ProjectConflictException] если имя занято (409)
  /// Throws [UnauthorizedException] если не авторизован (401)
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
      return ProjectModel.fromJson(response.data as Map<String, dynamic>);
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
      final json = request.toJson();
      json.removeWhere((_, value) => value == null);
      final response = await _dio.put(
        '/projects/$id',
        data: json,
        cancelToken: cancelToken,
      );
      return ProjectModel.fromJson(response.data as Map<String, dynamic>);
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
  ProjectRepositoryException _handleDioError(DioException error) {
    switch (error.type) {
      case DioExceptionType.badResponse:
        final statusCode = error.response?.statusCode;
        final data = error.response?.data;

        String? message;
        if (data is Map<String, dynamic>) {
          message = data['message'] as String? ?? data['error'] as String?;
        }
        final errorMsg = message ?? data.toString();

        switch (statusCode) {
          case 401:
            return UnauthorizedException(errorMsg);
          case 403:
            return ProjectForbiddenException(errorMsg);
          case 404:
            return ProjectNotFoundException(errorMsg);
          case 409:
            return ProjectConflictException(errorMsg);
          default:
            return ProjectApiException(
              '$statusCode: $errorMsg',
              statusCode: statusCode,
              originalError: error,
            );
        }

      case DioExceptionType.connectionTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.sendTimeout:
        return ProjectApiException('Network timeout', originalError: error);

      case DioExceptionType.connectionError:
        return ProjectApiException('Network error', originalError: error);

      default:
        return ProjectApiException(
          error.message ?? 'Unknown error',
          originalError: error,
        );
    }
  }
}
