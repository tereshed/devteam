import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mockito/mockito.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';

class MockDio implements Dio {
  final Map<String, dynamic> _responses = {};
  final Map<String, Exception> _errors = {};

  void setResponse(String key, dynamic response) {
    _responses[key] = response;
  }

  void setError(String key, Exception error) {
    _errors[key] = error;
  }

  @override
  String get baseUrl => 'http://127.0.0.1:8080/api/v1';

  @override
  set baseUrl(String url) {}

  @override
  Future<Response<T>> get<T>(
    String path, {
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
    ProgressCallback? onReceiveProgress,
  }) async {
    if (_errors.containsKey(path)) {
      throw _errors[path]!;
    }
    if (_responses.containsKey(path)) {
      return Response<T>(
        data: _responses[path],
        statusCode: 200,
        requestOptions: RequestOptions(path: path),
      );
    }
    return Response<T>(
      data: {} as T,
      statusCode: 200,
      requestOptions: RequestOptions(path: path),
    );
  }

  @override
  Future<Response<T>> post<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    if (_errors.containsKey(path)) {
      throw _errors[path]!;
    }
    if (_responses.containsKey(path)) {
      return Response<T>(
        data: _responses[path],
        statusCode: 201,
        requestOptions: RequestOptions(path: path, method: 'POST'),
      );
    }
    return Response<T>(
      data: {} as T,
      statusCode: 201,
      requestOptions: RequestOptions(path: path, method: 'POST'),
    );
  }

  @override
  Future<Response<T>> put<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
    ProgressCallback? onSendProgress,
    ProgressCallback? onReceiveProgress,
  }) async {
    if (_errors.containsKey(path)) {
      throw _errors[path]!;
    }
    if (_responses.containsKey(path)) {
      return Response<T>(
        data: _responses[path],
        statusCode: 200,
        requestOptions: RequestOptions(path: path, method: 'PUT'),
      );
    }
    return Response<T>(
      data: {} as T,
      statusCode: 200,
      requestOptions: RequestOptions(path: path, method: 'PUT'),
    );
  }

  @override
  Future<Response<T>> delete<T>(
    String path, {
    dynamic data,
    Map<String, dynamic>? queryParameters,
    Options? options,
    CancelToken? cancelToken,
  }) async {
    if (_errors.containsKey(path)) {
      throw _errors[path]!;
    }
    return Response<T>(
      data: null as T,
      statusCode: 204,
      requestOptions: RequestOptions(path: path, method: 'DELETE'),
    );
  }

  @override
  noSuchMethod(Invocation invocation) {
    return super.noSuchMethod(invocation);
  }

  @override
  Future<Response<dynamic>> getUri(Uri uri,
      {Options? options, CancelToken? cancelToken}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> postUri(Uri uri,
      {dynamic data,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      ProgressCallback? onReceiveProgress}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> putUri(Uri uri,
      {dynamic data,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      ProgressCallback? onReceiveProgress}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> deleteUri(Uri uri,
      {dynamic data,
      Options? options,
      CancelToken? cancelToken}) {
    throw UnimplementedError();
  }

  @override
  HttpClientAdapter get httpClientAdapter => throw UnimplementedError();

  @override
  set httpClientAdapter(HttpClientAdapter value) {}

  @override
  Transformer get transformer => throw UnimplementedError();

  @override
  set transformer(Transformer value) {}

  @override
  int get connectTimeout => 0;

  @override
  set connectTimeout(int ms) {}

  @override
  int get receiveTimeout => 0;

  @override
  set receiveTimeout(int ms) {}

  @override
  int get sendTimeout => 0;

  @override
  set sendTimeout(int ms) {}

  @override
  Interceptors get interceptors => throw UnimplementedError();

  @override
  Future<Response<dynamic>> patch(String path,
      {dynamic data,
      Map<String, dynamic>? queryParameters,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      ProgressCallback? onReceiveProgress}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> patchUri(Uri uri,
      {dynamic data,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      ProgressCallback? onReceiveProgress}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> head(String path,
      {Options? options, CancelToken? cancelToken}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> headUri(Uri uri,
      {Options? options, CancelToken? cancelToken}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> download(String urlPath, savePath,
      {ProgressCallback? onReceiveProgress,
      Map<String, dynamic>? queryParameters,
      CancelToken? cancelToken,
      bool deleteOnError = true,
      String lengthHeader = 'content-length',
      dynamic data,
      Options? options}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> downloadUri(Uri uri, savePath,
      {ProgressCallback? onReceiveProgress,
      CancelToken? cancelToken,
      bool deleteOnError = true,
      String lengthHeader = 'content-length',
      dynamic data,
      Options? options}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> uploadMultiple(String urlPath, requests,
      {CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      Options? options}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> uploadMultipleUri(Uri uri, requests,
      {CancelToken? cancelToken,
      ProgressCallback? onSendProgress,
      Options? options}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> uploadStream(String urlPath,
      Stream<List<int>> fileStream, int length,
      {String? filename,
      Map<String, dynamic>? queryParameters,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> uploadStreamUri(Uri uri,
      Stream<List<int>> fileStream, int length,
      {String? filename,
      Options? options,
      CancelToken? cancelToken,
      ProgressCallback? onSendProgress}) {
    throw UnimplementedError();
  }
}

void main() {
  late MockDio mockDio;
  late ProjectRepository repository;

  const projectJson = {
    'id': '123e4567-e89b-12d3-a456-426614174000',
    'name': 'Test Project',
    'description': 'A test project',
    'git_provider': 'github',
    'git_url': 'https://github.com/user/repo.git',
    'git_default_branch': 'main',
    'git_credential': null,
    'vector_collection': 'test_project',
    'tech_stack': {'backend': 'Go'},
    'status': 'active',
    'settings': {},
    'created_at': '2026-04-28T10:00:00Z',
    'updated_at': '2026-04-28T10:00:00Z',
  };

  setUp(() {
    mockDio = MockDio();
    repository = ProjectRepository(dio: mockDio);
  });

  group('getProject', () {
    test('test_getProject_success', () async {
      when(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async =>
          Response(
            data: projectJson,
            statusCode: 200,
            requestOptions: RequestOptions(
              path: '/projects/123e4567-e89b-12d3-a456-426614174000',
            ),
          ) as Response<Map<String, dynamic>>);

      final result =
          await repository.getProject('123e4567-e89b-12d3-a456-426614174000');

      expect(result.id, '123e4567-e89b-12d3-a456-426614174000');
      expect(result.name, 'Test Project');
      verify(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_getProject_unauthorized', () async {
      when(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/123e4567-e89b-12d3-a456-426614174000',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'unauthorized', 'message': 'not authorized'},
          statusCode: 401,
          requestOptions: RequestOptions(
            path: '/projects/123e4567-e89b-12d3-a456-426614174000',
          ),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.getProject('123e4567-e89b-12d3-a456-426614174000'),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('test_getProject_forbidden', () async {
      when(mockDio.get(
        '/projects/other-user-id',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/other-user-id'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'forbidden', 'message': 'no access to this project'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/projects/other-user-id'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.getProject('other-user-id'),
        throwsA(isA<ProjectForbiddenException>()),
      );
    });

    test('test_getProject_notFound', () async {
      when(mockDio.get(
        '/projects/nonexistent',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/nonexistent'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'not_found', 'message': 'project not found'},
          statusCode: 404,
          requestOptions: RequestOptions(path: '/projects/nonexistent'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.getProject('nonexistent'),
        throwsA(isA<ProjectNotFoundException>()),
      );
    });

    test('test_getProject_emptyId', () {
      expect(
        () => repository.getProject(''),
        throwsA(isA<ArgumentError>()),
      );
    });
  });

  group('listProjects', () {
    test('test_listProjects_success', () async {
      final listJson = {
        'projects': [projectJson],
        'total': 1,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: anyNamed('queryParameters'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      final result = await repository.listProjects();

      expect(result.projects, isNotEmpty);
      expect(result.total, 1);
      expect(result.limit, 20);
      expect(result.offset, 0);
      verify(mockDio.get(
        '/projects',
        queryParameters: anyNamed('queryParameters'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_listProjects_empty', () async {
      final listJson = {
        'projects': [],
        'total': 0,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: anyNamed('queryParameters'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      final result = await repository.listProjects();

      expect(result.projects, isEmpty);
      expect(result.total, 0);
    });

    test('test_listProjects_withFilter', () async {
      final listJson = {
        'projects': [],
        'total': 0,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: argThat(
          allOf([
            containsPair('status', 'active'),
            containsPair('git_provider', 'github'),
          ]),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      final filter = ProjectListFilter(
        status: 'active',
        gitProvider: 'github',
      );

      await repository.listProjects(filter: filter);

      verify(mockDio.get(
        '/projects',
        queryParameters: argThat(
          allOf([
            containsPair('status', 'active'),
            containsPair('git_provider', 'github'),
          ]),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_listProjects_normalizesLimit', () async {
      final listJson = {
        'projects': [],
        'total': 0,
        'limit': 100,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('limit', 100),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      await repository.listProjects(limit: 150);

      verify(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('limit', 100),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_listProjects_defaultLimit', () async {
      final listJson = {
        'projects': [],
        'total': 0,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('limit', 20),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      await repository.listProjects(limit: 0);

      verify(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('limit', 20),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_listProjects_negativeOffsetNormalized', () async {
      final listJson = {
        'projects': [],
        'total': 0,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('offset', 0),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: listJson,
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      await repository.listProjects(offset: -5);

      verify(mockDio.get(
        '/projects',
        queryParameters: argThat(
          containsPair('offset', 0),
          named: 'queryParameters',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });
  });

  group('createProject', () {
    test('test_createProject_success', () async {
      final request = CreateProjectRequest(
        name: 'New Project',
        gitProvider: 'github',
        gitUrl: 'https://github.com/user/repo.git',
        vectorCollection: 'new_project',
      );

      when(mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: projectJson,
            statusCode: 201,
            requestOptions: RequestOptions(path: '/projects', method: 'POST'),
          ));

      final result = await repository.createProject(request);

      expect(result.id, '123e4567-e89b-12d3-a456-426614174000');
      expect(result.name, 'Test Project');
      verify(mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_createProject_badRequest', () async {
      final request = CreateProjectRequest(
        name: 'Invalid',
        gitProvider: 'github',
        gitUrl: 'invalid-url',
        vectorCollection: 'test',
      );

      when(mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'bad_request', 'message': 'invalid git url'},
          statusCode: 400,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.createProject(request),
        throwsA(isA<ProjectApiException>()),
      );
    });

    test('test_createProject_conflict', () async {
      final request = CreateProjectRequest(
        name: 'Duplicate Name',
        gitProvider: 'github',
        gitUrl: 'https://github.com/user/repo.git',
        vectorCollection: 'test',
      );

      when(mockDio.post(
        '/projects',
        data: argThat(
          predicate<Map<String, dynamic>>((m) => m['name'] == 'Duplicate Name'),
          named: 'data',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {
            'error': 'duplicate',
            'message': 'project name already exists'
          },
          statusCode: 409,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.createProject(request),
        throwsA(isA<ProjectConflictException>()),
      );
    });

    test('test_createProject_forbidden', () async {
      final request = CreateProjectRequest(
        name: 'Test',
        gitProvider: 'github',
        gitUrl: 'https://github.com/user/repo.git',
        vectorCollection: 'test',
      );

      when(mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'forbidden', 'message': 'no permission to create'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.createProject(request),
        throwsA(isA<ProjectForbiddenException>()),
      );
    });
  });

  group('updateProject', () {
    test('test_updateProject_success', () async {
      final request = UpdateProjectRequest(
        name: 'Updated Name',
        description: 'Updated description',
      );

      when(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: {...projectJson, 'name': 'Updated Name'},
            statusCode: 200,
            requestOptions: RequestOptions(
              path: '/projects/123e4567-e89b-12d3-a456-426614174000',
              method: 'PUT',
            ),
          ));

      final result = await repository.updateProject(
        '123e4567-e89b-12d3-a456-426614174000',
        request,
      );

      expect(result.name, 'Updated Name');
    });

    test('test_updateProject_badRequest', () async {
      final request = UpdateProjectRequest(gitUrl: 'invalid-url');

      when(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/123e4567-e89b-12d3-a456-426614174000',
          method: 'PUT',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'bad_request', 'message': 'invalid git url'},
          statusCode: 400,
          requestOptions:
              RequestOptions(path: '/projects/123e4567-e89b-12d3-a456-426614174000'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.updateProject(
          '123e4567-e89b-12d3-a456-426614174000',
          request,
        ),
        throwsA(isA<ProjectApiException>()),
      );
    });

    test('test_updateProject_notFound', () async {
      final request = UpdateProjectRequest(name: 'Updated');

      when(mockDio.put(
        '/projects/nonexistent',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/nonexistent', method: 'PUT'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'not_found', 'message': 'project not found'},
          statusCode: 404,
          requestOptions: RequestOptions(path: '/projects/nonexistent'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.updateProject('nonexistent', request),
        throwsA(isA<ProjectNotFoundException>()),
      );
    });

    test('test_updateProject_conflict', () async {
      final request = UpdateProjectRequest(name: 'Duplicate Name');

      when(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/123e4567-e89b-12d3-a456-426614174000',
          method: 'PUT',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'conflict', 'message': 'name already exists'},
          statusCode: 409,
          requestOptions:
              RequestOptions(path: '/projects/123e4567-e89b-12d3-a456-426614174000'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.updateProject(
          '123e4567-e89b-12d3-a456-426614174000',
          request,
        ),
        throwsA(isA<ProjectConflictException>()),
      );
    });

    test('test_updateProject_forbidden', () async {
      final request = UpdateProjectRequest(name: 'Updated');

      when(mockDio.put(
        '/projects/other-user-id',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions:
            RequestOptions(path: '/projects/other-user-id', method: 'PUT'),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'forbidden', 'message': 'no access'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/projects/other-user-id'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.updateProject('other-user-id', request),
        throwsA(isA<ProjectForbiddenException>()),
      );
    });

    test('test_updateProject_emptyId', () {
      final request = UpdateProjectRequest(name: 'Updated');

      expect(
        () => repository.updateProject('', request),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('test_updateProject_conflictingFlags', () {
      final request = UpdateProjectRequest(
        gitCredentialId: 'some-id',
        removeGitCredential: true,
      );

      expect(
        () => repository.updateProject('123e4567-e89b-12d3-a456-426614174000', request),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('test_updateProject_partialUpdate', () async {
      final request = UpdateProjectRequest(
        name: 'New Name',
        description: 'New Description',
      );

      when(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: argThat(
          predicate<Map<String, dynamic>>((m) {
            return m.containsKey('name') &&
                m.containsKey('description') &&
                !m.containsKey('git_provider') &&
                !m.containsKey('git_url') &&
                !m.containsKey('status');
          }),
          named: 'data',
        ),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: projectJson,
            statusCode: 200,
            requestOptions: RequestOptions(
              path: '/projects/123e4567-e89b-12d3-a456-426614174000',
              method: 'PUT',
            ),
          ));

      await repository.updateProject(
        '123e4567-e89b-12d3-a456-426614174000',
        request,
      );

      verify(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });
  });

  group('deleteProject', () {
    test('test_deleteProject_success', () async {
      when(mockDio.delete(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<dynamic>(
            data: null,
            statusCode: 204,
            requestOptions: RequestOptions(
              path: '/projects/123e4567-e89b-12d3-a456-426614174000',
              method: 'DELETE',
            ),
          ));

      await repository.deleteProject('123e4567-e89b-12d3-a456-426614174000');

      verify(mockDio.delete(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_deleteProject_unauthorized', () async {
      when(mockDio.delete(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/123e4567-e89b-12d3-a456-426614174000',
          method: 'DELETE',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'unauthorized', 'message': 'not authorized'},
          statusCode: 401,
          requestOptions:
              RequestOptions(path: '/projects/123e4567-e89b-12d3-a456-426614174000'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.deleteProject('123e4567-e89b-12d3-a456-426614174000'),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('test_deleteProject_forbidden', () async {
      when(mockDio.delete(
        '/projects/other-user-id',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/other-user-id',
          method: 'DELETE',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'forbidden', 'message': 'no access'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/projects/other-user-id'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.deleteProject('other-user-id'),
        throwsA(isA<ProjectForbiddenException>()),
      );
    });

    test('test_deleteProject_notFound', () async {
      when(mockDio.delete(
        '/projects/nonexistent',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(
          path: '/projects/nonexistent',
          method: 'DELETE',
        ),
        response: Response<Map<String, dynamic>>(
          data: {'error': 'not_found', 'message': 'project not found'},
          statusCode: 404,
          requestOptions: RequestOptions(path: '/projects/nonexistent'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.deleteProject('nonexistent'),
        throwsA(isA<ProjectNotFoundException>()),
      );
    });

    test('test_deleteProject_emptyId', () {
      expect(
        () => repository.deleteProject(''),
        throwsA(isA<ArgumentError>()),
      );
    });
  });

  group('Network errors', () {
    test('test_networkError_timeout', () async {
      when(mockDio.get(
        '/projects/123',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/123'),
        type: DioExceptionType.receiveTimeout,
        message: 'Receive timeout',
      ));

      expect(
        () => repository.getProject('123'),
        throwsA(isA<ProjectApiException>()),
      );
    });

    test('test_networkError_connectionError', () async {
      when(mockDio.get(
        '/projects/123',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/123'),
        type: DioExceptionType.connectionError,
        message: 'Connection error',
      ));

      expect(
        () => repository.getProject('123'),
        throwsA(isA<ProjectApiException>()),
      );
    });
  });
}
