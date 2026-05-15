// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_exceptions.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_repository.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'orchestration_v2_repository_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late OrchestrationV2Repository repository;

  const taskId = '11111111-1111-1111-1111-111111111111';
  const artifactId = '22222222-2222-2222-2222-222222222222';

  setUp(() {
    mockDio = MockDio();
    repository = OrchestrationV2Repository(dio: mockDio);
  });

  group('getArtifact', () {
    test('200: парсит Artifact с заполненным content (code_diff)', () async {
      final body = <String, dynamic>{
        'id': artifactId,
        'task_id': taskId,
        'producer_agent': 'developer',
        'kind': 'code_diff',
        'summary': '+12/-3 lines',
        'status': 'ready',
        'iteration': 1,
        'created_at': '2026-05-15T10:00:00Z',
        'content': <String, dynamic>{
          'diff': '--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-a\n+b\n',
        },
      };

      when(mockDio.get<dynamic>(
        '/tasks/$taskId/artifacts/$artifactId',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<dynamic>(
            data: body,
            statusCode: 200,
            requestOptions:
                RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
          ));

      final r = await repository.getArtifact(taskId, artifactId);
      expect(r.id, artifactId);
      expect(r.kind, 'code_diff');
      expect(r.content, isNotNull);
      expect(r.content!['diff'], contains('@@'));
    });

    test('200 + не Map: бросает OrchestrationV2ApiException', () async {
      when(mockDio.get<dynamic>(
        '/tasks/$taskId/artifacts/$artifactId',
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => Response<dynamic>(
            data: 'not a json object',
            statusCode: 200,
            requestOptions:
                RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
          ));

      expect(
        () => repository.getArtifact(taskId, artifactId),
        throwsA(isA<OrchestrationV2ApiException>()),
      );
    });

    test('404: бросает OrchestrationV2NotFoundException', () async {
      when(mockDio.get<dynamic>(
        '/tasks/$taskId/artifacts/$artifactId',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions:
            RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
        response: Response<dynamic>(
          statusCode: 404,
          data: <String, dynamic>{'error': 'artifact not found'},
          requestOptions:
              RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.getArtifact(taskId, artifactId),
        throwsA(isA<OrchestrationV2NotFoundException>()),
      );
    });

    test('500: бросает OrchestrationV2ApiException', () async {
      when(mockDio.get<dynamic>(
        '/tasks/$taskId/artifacts/$artifactId',
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(DioException(
        requestOptions:
            RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
        response: Response<dynamic>(
          statusCode: 500,
          requestOptions:
              RequestOptions(path: '/tasks/$taskId/artifacts/$artifactId'),
        ),
        type: DioExceptionType.badResponse,
      ));

      expect(
        () => repository.getArtifact(taskId, artifactId),
        throwsA(isA<OrchestrationV2ApiException>()),
      );
    });
  });
}
