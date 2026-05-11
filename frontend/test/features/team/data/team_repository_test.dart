import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/team/data/team_repository.dart';
import 'package:frontend/features/team/domain/team_exceptions.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'team_repository_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late TeamRepository repository;

  const projectId = '550e8400-e29b-41d4-a716-446655440000';

  Map<String, dynamic> teamJson({String? projectIdOverride}) {
    return <String, dynamic>{
      'id': 'team-1',
      'name': 'Dev Team',
      'project_id': projectIdOverride ?? projectId,
      'type': 'development',
      'agents': <Map<String, dynamic>>[],
      'created_at': '2026-04-27T09:00:00Z',
      'updated_at': '2026-04-27T09:15:00Z',
    };
  }

  setUp(() {
    mockDio = MockDio();
    repository = TeamRepository(dio: mockDio);
  });

  group('getTeam', () {
    test('success', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
        ),
      );

      final team = await repository.getTeam(projectId);
      expect(team.name, 'Dev Team');
      expect(team.projectId, projectId);
      verify(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('empty projectId throws ArgumentError', () async {
      expect(
        () => repository.getTeam(''),
        throwsArgumentError,
      );
      verifyNever(mockDio.get(any, cancelToken: anyNamed('cancelToken')));
    });

    test('projectId mismatch throws TeamProjectMismatchException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(projectIdOverride: 'other-uuid'),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(isA<TeamProjectMismatchException>()),
      );
    });

    test('401 UnauthorizedException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'unauthorized', 'message': 'nope'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'unauthorized',
          ),
        ),
      );
    });

    test('403 TeamForbiddenException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'forbidden', 'message': 'no'},
            statusCode: 403,
            requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(isA<TeamForbiddenException>()),
      );
    });

    test('404 TeamNotFoundException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'missing'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(isA<TeamNotFoundException>()),
      );
    });

    test('409 maps to TeamConflictException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'conflict', 'message': 'x'},
            statusCode: 409,
            requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(isA<TeamConflictException>()),
      );
    });

    test('200 with null body throws TeamApiException (onInvalid)', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: null,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(
          isA<TeamApiException>().having(
            (e) => e.message,
            'message',
            'Empty response body',
          ),
        ),
      );
    });

    test('200 with non-object JSON throws TeamApiException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: <dynamic>[],
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(
          isA<TeamApiException>().having(
            (e) => e.message,
            'message',
            'Expected JSON object in response body',
          ),
        ),
      );
    });

    test('receiveTimeout → TeamApiException без statusCode', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          type: DioExceptionType.receiveTimeout,
          message: 'Receive timeout',
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(
          isA<TeamApiException>().having(
            (e) => e.statusCode,
            'statusCode',
            isNull,
          ),
        ),
      );
    });

    test('connectionError → TeamApiException без statusCode', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          type: DioExceptionType.connectionError,
          message: 'Connection error',
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(
          isA<TeamApiException>().having(
            (e) => e.statusCode,
            'statusCode',
            isNull,
          ),
        ),
      );
    });

    test('cancel TeamCancelledException', () async {
      when(
        mockDio.get(
          '/projects/$projectId/team',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/team'),
          type: DioExceptionType.cancel,
        ),
      );

      expect(
        () => repository.getTeam(projectId),
        throwsA(isA<TeamCancelledException>()),
      );
    });
  });

  group('patchAgent', () {
    const agentId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';

    test('success', () async {
      when(
        mockDio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
      );

      final team = await repository.patchAgent(
        projectId,
        agentId,
        <String, dynamic>{'is_active': false},
      );
      expect(team.name, 'Dev Team');
      verify(
        mockDio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: {'is_active': false},
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('409 TeamConflictException', () async {
      when(
        mockDio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'conflict', 'message': 'x'},
            statusCode: 409,
            requestOptions: RequestOptions(
              path: '/projects/$projectId/team/agents/$agentId',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.patchAgent(projectId, agentId, {'model': 'x'}),
        throwsA(isA<TeamConflictException>()),
      );
    });
  });
}
