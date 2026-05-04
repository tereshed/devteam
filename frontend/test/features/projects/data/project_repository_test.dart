import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'project_repository_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late ProjectRepository repository;

  Map<String, dynamic> getProjectJson() {
    return <String, dynamic>{
      'id': '123e4567-e89b-12d3-a456-426614174000',
      'name': 'Test Project',
      'description': 'A test project',
      'git_provider': 'github',
      'git_url': 'https://github.com/user/repo.git',
      'git_default_branch': 'main',
      'git_credential': null,
      'vector_collection': 'test_project',
      'tech_stack': <String, dynamic>{'backend': 'Go'},
      'status': 'active',
      'settings': <String, dynamic>{},
      'created_at': '2026-04-28T10:00:00Z',
      'updated_at': '2026-04-28T10:00:00Z',
    };
  }

  setUp(() {
    mockDio = MockDio();
    repository = ProjectRepository(dio: mockDio);
  });

  group('getProject', () {
    test('test_getProject_success', () async {
      when(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<dynamic>(
            data: Map<String, dynamic>.from(getProjectJson()),
            statusCode: 200,
            requestOptions: RequestOptions(
              path: '/projects/123e4567-e89b-12d3-a456-426614174000',
            ),
          ));

      final result =
          await repository.getProject('123e4567-e89b-12d3-a456-426614174000');

      expect(result.id, '123e4567-e89b-12d3-a456-426614174000');
      expect(result.name, 'Test Project');
      verify(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('test_getProject_emptyJsonBody_throwsProjectApiException', () async {
      when(mockDio.get(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
        (_) async => Response<dynamic>(
          data: null,
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/123e4567-e89b-12d3-a456-426614174000',
          ),
        ),
      );

      expect(
        () => repository.getProject('123e4567-e89b-12d3-a456-426614174000'),
        throwsA(
          isA<ProjectApiException>().having(
            (e) => e.message,
            'message',
            'Empty response body',
          ),
        ),
      );
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
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'unauthorized',
          ),
        ),
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
        throwsA(
          isA<ProjectForbiddenException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'forbidden',
          ),
        ),
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
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
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
      final listJson = <String, dynamic>{
        'projects': [getProjectJson()],
        'total': 1,
        'limit': 20,
        'offset': 0,
      };

      when(mockDio.get(
        '/projects',
        queryParameters: anyNamed('queryParameters'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: Map<String, dynamic>.from(listJson),
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
            data: Map<String, dynamic>.from(listJson),
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
            data: Map<String, dynamic>.from(listJson),
            statusCode: 200,
            requestOptions: RequestOptions(path: '/projects'),
          ));

      const filter = ProjectListFilter(
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
            data: Map<String, dynamic>.from(listJson),
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
            data: Map<String, dynamic>.from(listJson),
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
            data: Map<String, dynamic>.from(listJson),
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
      const request = CreateProjectRequest(
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
            data: Map<String, dynamic>.from(getProjectJson()),
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
      const request = CreateProjectRequest(
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
      const request = CreateProjectRequest(
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
        throwsA(
          isA<ProjectConflictException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'duplicate',
          ),
        ),
      );
    });

    test('test_createProject_forbidden', () async {
      const request = CreateProjectRequest(
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
        throwsA(
          isA<ProjectForbiddenException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'forbidden',
          ),
        ),
      );
    });

    test('test_createProject_error_message_sanitizes_url_userinfo', () async {
      const request = CreateProjectRequest(
        name: 'Test',
        gitProvider: 'github',
        gitUrl: 'https://github.com/user/repo.git',
        vectorCollection: '',
      );

      when(mockDio.post(
        '/projects',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects', method: 'POST'),
        response: Response<Map<String, dynamic>>(
          data: {
            'message':
                'clone failed https://u:sekrettoken@git.example/a.git extra',
          },
          statusCode: 400,
          requestOptions: RequestOptions(path: '/projects'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.createProject(request),
        throwsA(
          predicate<ProjectApiException>((e) {
            expect(e.message, isNot(contains('sekrettoken')));
            expect(e.message, isNot(contains('u:sekrettoken')));
            return true;
          }),
        ),
      );
    });
  });

  group('updateProject', () {
    test('test_updateProject_success', () async {
      const request = UpdateProjectRequest(
        name: 'Updated Name',
        description: 'Updated description',
      );

      when(mockDio.put(
        '/projects/123e4567-e89b-12d3-a456-426614174000',
        data: anyNamed('data'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: <String, dynamic>{...Map<String, dynamic>.from(getProjectJson()), 'name': 'Updated Name'},
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
      const request = UpdateProjectRequest(gitUrl: 'invalid-url');

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
      const request = UpdateProjectRequest(name: 'Updated');

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
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test('test_updateProject_conflict', () async {
      const request = UpdateProjectRequest(name: 'Duplicate Name');

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
        throwsA(
          isA<ProjectConflictException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'conflict',
          ),
        ),
      );
    });

    test('test_updateProject_forbidden', () async {
      const request = UpdateProjectRequest(name: 'Updated');

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
        throwsA(
          isA<ProjectForbiddenException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'forbidden',
          ),
        ),
      );
    });

    test('test_updateProject_emptyId', () {
      const request = UpdateProjectRequest(name: 'Updated');

      expect(
        () => repository.updateProject('', request),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('test_updateProject_conflictingFlags', () {
      const request = UpdateProjectRequest(
        gitCredentialId: 'some-id',
        removeGitCredential: true,
      );

      expect(
        () => repository.updateProject('123e4567-e89b-12d3-a456-426614174000', request),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('test_updateProject_partialUpdate', () async {
      const request = UpdateProjectRequest(
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
            data: Map<String, dynamic>.from(getProjectJson()),
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
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'unauthorized',
          ),
        ),
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
        throwsA(
          isA<ProjectForbiddenException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'forbidden',
          ),
        ),
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
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
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

    test('test_networkError_cancel', () async {
      when(mockDio.get(
        '/projects/123',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/projects/123'),
        type: DioExceptionType.cancel,
      ));

      expect(
        () => repository.getProject('123'),
        throwsA(isA<ProjectCancelledException>()),
      );
    });
  });
}
