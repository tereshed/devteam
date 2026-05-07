// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'task_repository_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  late MockDio mockDio;
  late TaskRepository repository;

  const projectId = '11111111-1111-1111-1111-111111111111';
  const taskId = '22222222-2222-2222-2222-222222222222';
  const userId = '33333333-3333-3333-3333-333333333333';

  Map<String, dynamic> taskListItemJson() => <String, dynamic>{
        'id': taskId,
        'project_id': projectId,
        'title': 'Task',
        'status': 'pending',
        'priority': 'medium',
        'created_by_type': 'user',
        'created_by_id': userId,
        'created_at': '2026-05-07T10:00:00Z',
        'updated_at': '2026-05-07T10:00:00Z',
      };

  Map<String, dynamic> taskFullJson() => <String, dynamic>{
        'id': taskId,
        'project_id': projectId,
        'title': 'Task',
        'description': '',
        'status': 'pending',
        'priority': 'medium',
        'created_by_type': 'user',
        'created_by_id': userId,
        'context': <String, dynamic>{},
        'artifacts': <String, dynamic>{},
        'sub_tasks': <Map<String, dynamic>>[],
        'created_at': '2026-05-07T10:00:00Z',
        'updated_at': '2026-05-07T10:00:00Z',
      };

  Map<String, dynamic> taskMessageJson() => <String, dynamic>{
        'id': '44444444-4444-4444-4444-444444444444',
        'task_id': taskId,
        'sender_type': 'user',
        'sender_id': userId,
        'content': 'hello',
        'message_type': 'instruction',
        'metadata': <String, dynamic>{},
        'created_at': '2026-05-07T10:01:00Z',
      };

  setUp(() {
    mockDio = MockDio();
    repository = TaskRepository(dio: mockDio);
  });

  group('listTasks', () {
    test('200 parses TaskListResponse', () async {
      final body = <String, dynamic>{
        'tasks': [taskListItemJson()],
        'total': 1,
        'limit': 50,
        'offset': 0,
      };

      when(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          queryParameters: anyNamed('queryParameters'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: body,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
        ),
      );

      final r = await repository.listTasks(projectId);

      expect(r.tasks, hasLength(1));
      expect(r.total, 1);
      expect(r.tasks.first.id, taskId);
    });

    test('normalizes limit and offset (50 / 200 / 0)', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          queryParameters: anyNamed('queryParameters'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'tasks': <Map<String, dynamic>>[],
            'total': 0,
            'limit': 200,
            'offset': 0,
          },
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
        ),
      );

      await repository.listTasks(projectId, limit: 999, offset: -3);

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          queryParameters: argThat(
            allOf([
              containsPair('limit', 200),
              containsPair('offset', 0),
            ]),
            named: 'queryParameters',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('statuses [A, B] uses multi query + ListFormat.multi', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'tasks': <Map<String, dynamic>>[],
            'total': 0,
            'limit': 50,
            'offset': 0,
          },
          statusCode: 200,
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
        ),
      );

      await repository.listTasks(
        projectId,
        filter: const TaskListFilter(
          statuses: ['pending', 'in_progress'],
        ),
      );

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          queryParameters: argThat(
            containsPair('statuses', ['pending', 'in_progress']),
            named: 'queryParameters',
          ),
          options: argThat(
            predicate<Options>((o) => o.listFormat == ListFormat.multi),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('404 -> ProjectNotFoundException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'project not found'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listTasks(projectId),
        throwsA(
          isA<ProjectNotFoundException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'not_found',
          ),
        ),
      );
    });

    test('403 on /projects/.../tasks -> TaskForbiddenException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'forbidden', 'message': 'no access'},
            statusCode: 403,
            requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listTasks(projectId),
        throwsA(isA<TaskForbiddenException>()),
      );
    });
  });

  group('createTask', () {
    test('201 + Options(contentType: application/json)', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 201,
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
        ),
      );

      await repository.createTask(
        projectId,
        const CreateTaskRequest(title: 'New'),
      );

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          data: argThat(
            containsPair('title', 'New'),
            named: 'data',
          ),
          options: argThat(
            predicate<Options>((o) => o.contentType == 'application/json'),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('createTask JSON без null optional UUID (includeIfNull: false)', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 201,
          requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
        ),
      );

      await repository.createTask(
        projectId,
        const CreateTaskRequest(title: 'OnlyTitle'),
      );

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/projects/$projectId/tasks',
          data: argThat(
            allOf([
              containsPair('title', 'OnlyTitle'),
              predicate<Map<String, dynamic>>(
                (m) =>
                    !m.containsKey('parent_task_id') &&
                    !m.containsKey('assigned_agent_id'),
              ),
            ]),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });

  group('getTask / updateTask / mutations', () {
    test('getTask 200', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      final t = await repository.getTask(taskId);
      expect(t.id, taskId);
    });

    test('getTask 200 с не-map телом -> TaskApiException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: null,
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(isA<TaskApiException>()),
      );
    });

    test('updateTask 200 (PUT) + TaskModel', () async {
      when(
        mockDio.put<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      final t = await repository.updateTask(
        taskId,
        const UpdateTaskRequest(title: 'x'),
      );
      expect(t.id, taskId);
    });
  });

  group('404 /tasks/...', () {
    test('getTask 404 -> TaskNotFoundException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found', 'message': 'missing'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/tasks/$taskId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(isA<TaskNotFoundException>()),
      );
    });

    test('listTaskMessages 404 -> TaskNotFoundException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'not_found'},
            statusCode: 404,
            requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.listTaskMessages(taskId),
        throwsA(isA<TaskNotFoundException>()),
      );
    });
  });

  group('deleteTask', () {
    test('204', () async {
      when(
        mockDio.delete<void>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<void>(
          statusCode: 204,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      await repository.deleteTask(taskId);

      verify(
        mockDio.delete<void>(
          '/tasks/$taskId',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });

  group('listTaskMessages / addTaskMessage', () {
    test('200 list', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'messages': [taskMessageJson()],
            'total': 1,
            'limit': 50,
            'offset': 0,
          },
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
        ),
      );

      final r = await repository.listTaskMessages(taskId);
      expect(r.messages, hasLength(1));
    });

    test('listTaskMessages не передаёт Options(listFormat: multi)', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          queryParameters: anyNamed('queryParameters'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: <String, dynamic>{
            'messages': <dynamic>[],
            'total': 0,
            'limit': 50,
            'offset': 0,
          },
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
        ),
      );

      await repository.listTaskMessages(taskId);

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/tasks/$taskId/messages',
          queryParameters: anyNamed('queryParameters'),
          options: argThat(isNull, named: 'options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('201 add + content-type', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskMessageJson(),
          statusCode: 201,
          requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
        ),
      );

      await repository.addTaskMessage(
        taskId,
        const CreateTaskMessageRequest(
          content: 'c',
          messageType: 'instruction',
        ),
      );

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/messages',
          data: anyNamed('data'),
          options: argThat(
            predicate<Options>((o) => o.contentType == 'application/json'),
            named: 'options',
          ),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });

  group('pause / cancel / resume', () {
    test('pauseTask: POST без data и без Options', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/pause'),
        ),
      );

      await repository.pauseTask(taskId);

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/pause',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('cancelTask: POST без data и без Options', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/cancel'),
        ),
      );

      await repository.cancelTask(taskId);

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/cancel',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('resumeTask: POST без data и без Options', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/resume'),
        ),
      );

      await repository.resumeTask(taskId);

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/resume',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('pauseTask 409 -> TaskConflictException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId/pause'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'conflict', 'message': 'invalid transition'},
            statusCode: 409,
            requestOptions: RequestOptions(path: '/tasks/$taskId/pause'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.pauseTask(taskId),
        throwsA(isA<TaskConflictException>()),
      );
    });
  });

  group('HTTP error mapping', () {
    test('401 -> UnauthorizedException by statusCode', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'access_denied', 'message': 'go away'},
            statusCode: 401,
            requestOptions: RequestOptions(path: '/tasks/$taskId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(
          isA<UnauthorizedException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'access_denied',
          ),
        ),
      );
    });

    test('403 getTask -> TaskForbiddenException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'forbidden'},
            statusCode: 403,
            requestOptions: RequestOptions(path: '/tasks/$taskId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(isA<TaskForbiddenException>()),
      );
    });

    test('409 -> TaskConflictException', () async {
      when(
        mockDio.put<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'conflict'},
            statusCode: 409,
            requestOptions: RequestOptions(path: '/tasks/$taskId'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.updateTask(
          taskId,
          const UpdateTaskRequest(title: 'x'),
        ),
        throwsA(
          isA<TaskConflictException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'conflict',
          ),
        ),
      );
    });

    test('422 -> TaskUnprocessableException', () async {
      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
          response: Response<Map<String, dynamic>>(
            data: {'error': 'unprocessable'},
            statusCode: 422,
            requestOptions: RequestOptions(path: '/tasks/$taskId/messages'),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      expect(
        () => repository.addTaskMessage(
          taskId,
          const CreateTaskMessageRequest(
            content: 'c',
            messageType: 'instruction',
          ),
        ),
        throwsA(
          isA<TaskUnprocessableException>().having(
            (e) => e.apiErrorCode,
            'apiErrorCode',
            'unprocessable',
          ),
        ),
      );
    });

    test('CancelToken cancel -> TaskCancelledException', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          type: DioExceptionType.cancel,
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(isA<TaskCancelledException>()),
      );
    });

    test('connectionError -> TaskApiException network', () async {
      when(
        mockDio.get<Map<String, dynamic>>(
          any,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
          type: DioExceptionType.connectionError,
        ),
      );

      expect(
        () => repository.getTask(taskId),
        throwsA(
          isA<TaskApiException>()
              .having((e) => e.statusCode, 'statusCode', isNull)
              .having((e) => e.isNetworkTransportError, 'network', isTrue),
        ),
      );
    });

    test('cancelToken пробрасывается в Dio', () async {
      final ct = CancelToken();
      when(
        mockDio.get<Map<String, dynamic>>(
          '/tasks/$taskId',
          cancelToken: ct,
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      await repository.getTask(taskId, cancelToken: ct);

      verify(
        mockDio.get<Map<String, dynamic>>(
          '/tasks/$taskId',
          cancelToken: ct,
        ),
      ).called(1);
    });
  });

  group('validation', () {
    test('пустой projectId / taskId — все методы с id', () async {
      expect(() => repository.listTasks(''), throwsArgumentError);
      expect(() => repository.createTask('', const CreateTaskRequest(title: 't')), throwsArgumentError);
      expect(() => repository.getTask(''), throwsArgumentError);
      expect(() => repository.updateTask('', const UpdateTaskRequest()), throwsArgumentError);
      expect(() => repository.deleteTask(''), throwsArgumentError);
      expect(() => repository.pauseTask(''), throwsArgumentError);
      expect(() => repository.cancelTask(''), throwsArgumentError);
      expect(() => repository.resumeTask(''), throwsArgumentError);
      expect(
        () => repository.correctTask('', const CorrectTaskRequest(text: 't')),
        throwsArgumentError,
      );
      expect(() => repository.listTaskMessages(''), throwsArgumentError);
      expect(
        () => repository.addTaskMessage(
          '',
          const CreateTaskMessageRequest(
            content: 'c',
            messageType: 'instruction',
          ),
        ),
        throwsArgumentError,
      );
    });

    test('invalid UUID projectId / taskId -> ArgumentError', () async {
      expect(
        () => repository.listTasks('not-a-uuid'),
        throwsArgumentError,
      );
      expect(
        () => repository.getTask('bad-id'),
        throwsArgumentError,
      );
    });

    test('createTask: невалидный parentTaskId / assignedAgentId -> ArgumentError', () async {
      expect(
        () => repository.createTask(
          projectId,
          const CreateTaskRequest(
            title: 't',
            parentTaskId: 'not-a-uuid',
          ),
        ),
        throwsArgumentError,
      );
      expect(
        () => repository.createTask(
          projectId,
          const CreateTaskRequest(
            title: 't',
            assignedAgentId: 'bad',
          ),
        ),
        throwsArgumentError,
      );
    });

    test('updateTask: невалидный assignedAgentId -> ArgumentError', () async {
      expect(
        () => repository.updateTask(
          taskId,
          const UpdateTaskRequest(assignedAgentId: 'not-a-uuid'),
        ),
        throwsArgumentError,
      );
    });

    test(
      'createTask / updateTask: пустая строка в optional UUID -> ArgumentError',
      () async {
        expect(
          () => repository.createTask(
            projectId,
            const CreateTaskRequest(
              title: 't',
              parentTaskId: '',
            ),
          ),
          throwsArgumentError,
        );
        expect(
          () => repository.createTask(
            projectId,
            const CreateTaskRequest(
              title: 't',
              assignedAgentId: '',
            ),
          ),
          throwsArgumentError,
        );
        expect(
          () => repository.updateTask(
            taskId,
            const UpdateTaskRequest(assignedAgentId: ''),
          ),
          throwsArgumentError,
        );
      },
    );

    test('correctTask empty text -> ArgumentError', () async {
      expect(
        () => repository.correctTask(
          taskId,
          const CorrectTaskRequest(text: ''),
        ),
        throwsArgumentError,
      );

      verifyNever(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/correct',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      );
    });

    test('correctTask whitespace-only -> ArgumentError, no POST /correct', () async {
      expect(
        () => repository.correctTask(
          taskId,
          const CorrectTaskRequest(text: '   \n\t '),
        ),
        throwsArgumentError,
      );

      verifyNever(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/correct',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      );
    });

    test('correctTask over limit -> ArgumentError, no POST /correct', () async {
      final huge = String.fromCharCodes(
        List<int>.generate(kUserCorrectionMaxBytes + 1, (_) => 97),
      );

      expect(
        () => repository.correctTask(
          taskId,
          CorrectTaskRequest(text: huge),
        ),
        throwsArgumentError,
      );

      verifyNever(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/correct',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      );
    });

    test('correctTask UTF-8: на границе байтов лимита (регрессия vs String.length)', () async {
      // «Я» = 2 байта UTF-8: число символов вдвое меньше байтового лимита — если считать только
      // String.length, верхняя граница будет ошибочно завышена (спека § «Частые ошибки» #9).
      final text =
          List<String>.filled(kUserCorrectionMaxBytes ~/ 2, 'Я').join();
      expect(text.length, lessThan(kUserCorrectionMaxBytes));
      expect(utf8.encode(text).length, lessThanOrEqualTo(kUserCorrectionMaxBytes));

      when(
        mockDio.post<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId/correct'),
        ),
      );

      await repository.correctTask(taskId, CorrectTaskRequest(text: text));

      verify(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/correct',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });

    test('correctTask: bytes > limit but chars < limit -> ArgumentError', () async {
      final overInBytes =
          List<String>.filled((kUserCorrectionMaxBytes ~/ 2) + 1, 'Я').join();
      expect(overInBytes.length, lessThan(kUserCorrectionMaxBytes));
      expect(
        utf8.encode(overInBytes).length,
        greaterThan(kUserCorrectionMaxBytes),
      );

      expect(
        () => repository.correctTask(
          taskId,
          CorrectTaskRequest(text: overInBytes),
        ),
        throwsArgumentError,
      );

      verifyNever(
        mockDio.post<Map<String, dynamic>>(
          '/tasks/$taskId/correct',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      );
    });

    test(
      'correctTask ASCII ровно kUserCorrectionMaxBytes байт -> 200',
      () async {
        final text = String.fromCharCodes(
          List<int>.filled(kUserCorrectionMaxBytes, 97),
        );
        expect(utf8.encode(text).length, kUserCorrectionMaxBytes);

        when(
          mockDio.post<Map<String, dynamic>>(
            any,
            data: anyNamed('data'),
            options: anyNamed('options'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => Response<Map<String, dynamic>>(
            data: taskFullJson(),
            statusCode: 200,
            requestOptions: RequestOptions(path: '/tasks/$taskId/correct'),
          ),
        );

        await repository.correctTask(taskId, CorrectTaskRequest(text: text));

        verify(
          mockDio.post<Map<String, dynamic>>(
            '/tasks/$taskId/correct',
            data: anyNamed('data'),
            options: anyNamed('options'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).called(1);
      },
    );
  });

  group('updateTask edge cases', () {
    test('updateTask assigned + clear -> ArgumentError', () async {
      expect(
        () => repository.updateTask(
          taskId,
          const UpdateTaskRequest(
            assignedAgentId: userId,
            clearAssignedAgent: true,
          ),
        ),
        throwsArgumentError,
      );
    });

    test(
      'updateTask только clearAssignedAgent -> PUT с clear_assigned_agent: true',
      () async {
        when(
          mockDio.put<Map<String, dynamic>>(
            any,
            data: anyNamed('data'),
            options: anyNamed('options'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => Response<Map<String, dynamic>>(
            data: taskFullJson(),
            statusCode: 200,
            requestOptions: RequestOptions(path: '/tasks/$taskId'),
          ),
        );

        await repository.updateTask(
          taskId,
          const UpdateTaskRequest(clearAssignedAgent: true),
        );

        verify(
          mockDio.put<Map<String, dynamic>>(
            '/tasks/$taskId',
            data: <String, dynamic>{'clear_assigned_agent': true},
            options: anyNamed('options'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).called(1);
      },
    );

    test('updateTask only title -> JSON без null-полей (includeIfNull: false)', () async {
      when(
        mockDio.put<Map<String, dynamic>>(
          any,
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: taskFullJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/tasks/$taskId'),
        ),
      );

      await repository.updateTask(
        taskId,
        const UpdateTaskRequest(title: 'only'),
      );

      verify(
        mockDio.put<Map<String, dynamic>>(
          '/tasks/$taskId',
          data: <String, dynamic>{
            'title': 'only',
            'clear_assigned_agent': false,
          },
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    });
  });
}
