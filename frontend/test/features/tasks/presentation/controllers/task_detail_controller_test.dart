@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/domain/ws_task_message_mapper.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:mockito/mockito.dart';

import 'task_list_controller_test.mocks.dart';
import '../../../../support/task_list_test_helpers.dart';

void main() {
  const pid = '550e8400-e29b-41d4-a716-446655440000';
  const tid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
  const uid = '33333333-3333-3333-3333-333333333333';

  late MockTaskRepository mockRepo;
  late ProviderContainer container;

  TaskModel task({
    String status = 'pending',
    DateTime? updatedAt,
    List<TaskSummaryModel> subs = const [],
    String? projectIdOverride,
    String? errorMessage,
  }) {
    return TaskModel(
      id: tid,
      projectId: projectIdOverride ?? pid,
      title: 't',
      description: '',
      status: status,
      priority: 'medium',
      createdByType: 'user',
      createdById: uid,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: updatedAt ?? DateTime.utc(2026, 1, 2),
      subTasks: subs,
      errorMessage: errorMessage,
    );
  }

  void stubList() {
    when(
      mockRepo.listTasks(
        pid,
        filter: anyNamed('filter'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => const TaskListResponse(tasks: [], total: 0, limit: 50, offset: 0),
    );
  }

  void stubMessages(TaskMessageListResponse response) {
    when(
      mockRepo.listTaskMessages(
        tid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => response);
  }

  Future<void> waitDetail() async {
    const step = Duration(milliseconds: 4);
    const timeout = Duration(seconds: 3);
    final sw = Stopwatch()..start();
    while (sw.elapsed < timeout) {
      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      if (st.hasError) {
        return;
      }
      if (st.hasValue &&
          !st.requireValue.isLoadingTask &&
          !st.requireValue.isLoadingMessages) {
        return;
      }
      await Future<void>.delayed(step);
    }
    fail('timeout waitDetail');
  }

  void listenKeepAlive() {
    container.listen(taskListControllerProvider(projectId: pid), (_, __) {});
    container.listen(taskDetailControllerProvider(projectId: pid, taskId: tid), (_, __) {});
  }

  setUp(() {
    mockRepo = MockTaskRepository();
    container = ProviderContainer(
      overrides: [
        taskRepositoryProvider.overrideWithValue(mockRepo),
      ],
    );
    addTearDown(container.dispose);
  });

  group('TaskDetailController', () {
    test('project mismatch → TaskDetailProjectMismatchException / AsyncError', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(projectIdOverride: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.hasError, isTrue);
      expect(st.error, isA<TaskDetailProjectMismatchException>());
    });

    test('pauseTask blockedByRealtime does not call repo', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      ctrl.setRealtimeMutationBlocked(true);
      final o = await ctrl.pauseTask();
      expect(o, TaskMutationOutcome.blockedByRealtime);
      verifyNever(mockRepo.pauseTask(any, cancelToken: anyNamed('cancelToken')));
    });

    test('correctTask empty trim → validationFailed before realtime check', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      ctrl.setRealtimeMutationBlocked(true);
      final o = await ctrl.correctTask('   ');
      expect(o, TaskMutationOutcome.validationFailed);
      verifyNever(mockRepo.correctTask(any, any, cancelToken: anyNamed('cancelToken')));
    });

    test('merge messages canonical order', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      final m1 = TaskMessageModel(
        id: '11111111-1111-1111-1111-111111111111',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'a',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 2),
      );
      final m2 = TaskMessageModel(
        id: '22222222-2222-2222-2222-222222222222',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'b',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 1),
      );
      stubMessages(
        TaskMessageListResponse(
          messages: [m1, m2],
          total: 2,
          limit: 50,
          offset: 0,
        ),
      );

      listenKeepAlive();
      container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final msgs =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.messages;
      expect(msgs.first.id, m2.id);
      expect(msgs.last.id, m1.id);
    });

    test('wsTaskMessageToModel copies metadata 1:1 without senderRole', () {
      final e = WsTaskMessageEvent(
        ts: DateTime.utc(2026, 5, 1),
        v: 1,
        projectId: pid,
        taskId: tid,
        messageId: '44444444-4444-4444-4444-444444444444',
        senderType: 'agent',
        senderId: uid,
        senderRole: 'developer',
        messageType: 'result',
        content: 'x',
        metadata: {'k': 1},
      );
      final m = wsTaskMessageToModel(e, taskId: tid);
      expect(m.metadata, {'k': 1});
      expect(m.createdAt, e.ts);
    });

    test('WS subtask patch updates only status', () async {
      const subId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).thenAnswer(
        (_) async => task(
          subs: [
            const TaskSummaryModel(
              id: subId,
              title: 'sub',
              status: 'pending',
              priority: 'low',
            ),
          ],
        ),
      );
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      container
          .read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier)
          .applyWsTaskStatus(
        WsTaskStatusEvent(
          ts: DateTime.utc(2026, 2, 1),
          v: 1,
          projectId: pid,
          taskId: subId,
          previousStatus: 'pending',
          status: 'in_progress',
          errorMessage: 'boom',
          assignedAgentId: '77777777-7777-7777-7777-777777777777',
        ),
      );

      final sub = container
          .read(taskDetailControllerProvider(projectId: pid, taskId: tid))
          .requireValue
          .task!
          .subTasks
          .single;
      expect(sub.status, 'in_progress');
      expect(sub.title, 'sub');
    });

    test('pauseTask when AsyncError → notReady', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(projectIdOverride: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).hasError,
        isTrue,
      );

      final o = await ctrl.pauseTask();
      expect(o, TaskMutationOutcome.notReady);
      verifyNever(mockRepo.pauseTask(any, cancelToken: anyNamed('cancelToken')));
    });

    test('WS previousStatus mismatch reloads task via getTask without extra messages fetch', () async {
      stubList();
      var msgCalls = 0;
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'in_progress'));
      when(
        mockRepo.listTaskMessages(
          tid,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        msgCalls++;
        return const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0);
      });

      listenKeepAlive();
      container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();
      expect(msgCalls, 1);

      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'paused'));

      container
          .read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier)
          .applyWsTaskStatus(
        WsTaskStatusEvent(
          ts: DateTime.utc(2026, 2, 1),
          v: 1,
          projectId: pid,
          taskId: tid,
          previousStatus: 'pending',
          status: 'paused',
        ),
      );

      await Future<void>.delayed(const Duration(milliseconds: 40));

      expect(msgCalls, 1);
      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task!.status,
        'paused',
      );
      verify(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).called(greaterThan(1));
    });

    test('applyWsTaskStatus clears parent errorMessage when event sends null', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'failed', errorMessage: 'old'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task!.errorMessage,
        'old',
      );

      ctrl.applyWsTaskStatus(
        WsTaskStatusEvent(
          ts: DateTime.utc(2026, 2, 2),
          v: 1,
          projectId: pid,
          taskId: tid,
          previousStatus: 'failed',
          status: 'in_progress',
          errorMessage: null,
        ),
      );

      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task!.errorMessage,
        isNull,
      );
    });

    test('applyRealtimeFailure terminal sets mutation block flag', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      ctrl.applyRealtimeFailure(const WsServiceFailure.authExpired());
      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.realtimeMutationBlocked,
        isTrue,
      );
    });

    test('applyRealtimeFailure(transient) keeps realtimeMutationBlocked false', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      ctrl.applyRealtimeFailure(const WsServiceFailure.transient());
      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.realtimeMutationBlocked,
        isFalse,
      );
    });

    test('deleteTask: taskDeleted, invalidate list triggers new listTasks', () async {
      var listCalls = 0;
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        listCalls++;
        return const TaskListResponse(tasks: [], total: 0, limit: 50, offset: 0);
      });
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();
      final afterDetail = listCalls;

      when(mockRepo.deleteTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async {});

      await ctrl.deleteTask();

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.requireValue.taskDeleted, isTrue);
      expect(st.requireValue.task, isNull);

      container.read(taskListControllerProvider(projectId: pid).notifier);
      await Future<void>.delayed(const Duration(milliseconds: 30));
      await waitTaskListControllerIdle(container, pid);

      expect(listCalls, greaterThan(afterDetail));
      verify(mockRepo.deleteTask(tid, cancelToken: anyNamed('cancelToken'))).called(1);
    });

    test('sendTaskMessage then WS same messageId → single merged row', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      const mid = '44444444-4444-4444-4444-444444444444';
      final httpMsg = TaskMessageModel(
        id: mid,
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'hello',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 3, 1),
      );
      when(
        mockRepo.addTaskMessage(tid, any, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => httpMsg);

      await ctrl.sendTaskMessage(
        const CreateTaskMessageRequest(content: 'hello', messageType: 'instruction'),
      );

      ctrl.applyWsTaskMessage(
        WsTaskMessageEvent(
          ts: DateTime.utc(2026, 3, 1),
          v: 1,
          projectId: pid,
          taskId: tid,
          messageId: mid,
          senderType: 'user',
          senderId: uid,
          messageType: 'instruction',
          content: 'hello',
        ),
      );

      final msgs =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.messages;
      expect(msgs.where((m) => m.id == mid), hasLength(1));
      expect(msgs, hasLength(1));
    });

    test('setMessageFilters resets totals and loads offset 0 with new messageType', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      verify(
        mockRepo.listTaskMessages(
          tid,
          messageType: null,
          senderType: null,
          limit: kTaskListDefaultLimit,
          offset: 0,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);

      when(
        mockRepo.listTaskMessages(
          tid,
          messageType: 'result',
          senderType: null,
          limit: kTaskListDefaultLimit,
          offset: 0,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskMessageListResponse(
          messages: [
            TaskMessageModel(
              id: '55555555-5555-5555-5555-555555555555',
              taskId: tid,
              senderType: 'agent',
              senderId: uid,
              content: 'r',
              messageType: 'result',
              createdAt: DateTime.utc(2026, 4, 1),
            ),
          ],
          total: 1,
          limit: 50,
          offset: 0,
        ),
      );

      await ctrl.setMessageFilters(messageType: 'result');

      final v = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(v.messageTypeFilter, 'result');
      expect(v.messagesOffset, 1);
      expect(v.messagesTotal, 1);
      expect(v.messages, hasLength(1));
    });

    test(
      'setMessageFilters: ошибка первой страницы лента не переводит провайдер в AsyncError',
      () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenKeepAlive();
        final ctrl =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        when(
          mockRepo.listTaskMessages(
            tid,
            messageType: 'result',
            senderType: null,
            limit: kTaskListDefaultLimit,
            offset: 0,
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenThrow(Exception('network'));

        await ctrl.setMessageFilters(messageType: 'result');

        final async = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
        expect(async.hasError, isFalse);
        final v = async.requireValue;
        expect(v.messagesLoadMoreError, isNotNull);
        expect(v.isLoadingMessages, isFalse);
      },
    );

    test('correctTask > kUserCorrectionMaxBytes UTF-8 → validationFailed', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final huge = List.filled(kUserCorrectionMaxBytes + 1, 'A').join();
      final o = await ctrl.correctTask(huge);
      expect(o, TaskMutationOutcome.validationFailed);
      verifyNever(
        mockRepo.correctTask(any, any, cancelToken: anyNamed('cancelToken')),
      );
    });

    test('updateTask assignedAgentId + clearAssignedAgent → validationFailed', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final o = await ctrl.updateTask(
        const UpdateTaskRequest(assignedAgentId: uid, clearAssignedAgent: true),
      );
      expect(o, TaskMutationOutcome.validationFailed);
      verifyNever(
        mockRepo.updateTask(any, any, cancelToken: anyNamed('cancelToken')),
      );
    });

    test('loadMoreMessages advances messagesOffset by page length (§53)', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task());

      final m0 = TaskMessageModel(
        id: '11111111-1111-1111-1111-111111111111',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'a',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 1),
      );
      final m1 = TaskMessageModel(
        id: '22222222-2222-2222-2222-222222222222',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'b',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 2),
      );
      final m2 = TaskMessageModel(
        id: '33333333-3333-3333-3333-333333333333',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'c',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 3),
      );
      final m3 = TaskMessageModel(
        id: '66666666-6666-6666-6666-666666666666',
        taskId: tid,
        senderType: 'user',
        senderId: uid,
        content: 'd',
        messageType: 'instruction',
        createdAt: DateTime.utc(2026, 1, 4),
      );

      when(
        mockRepo.listTaskMessages(
          tid,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((invocation) async {
        final offset = invocation.namedArguments[#offset] as int;
        if (offset == 0) {
          return TaskMessageListResponse(
            messages: [m0, m1],
            total: 5,
            limit: 50,
            offset: 0,
          );
        }
        if (offset == 2) {
          return TaskMessageListResponse(
            messages: [m2, m3],
            total: 5,
            limit: 50,
            offset: 2,
          );
        }
        return const TaskMessageListResponse(messages: [], total: 5, limit: 50, offset: 0);
      });

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.messagesOffset,
        2,
      );

      await ctrl.loadMoreMessages();

      final v = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(v.messagesOffset, 4);
      expect(v.messages.map((m) => m.id).toSet(), {m0.id, m1.id, m2.id, m3.id});
    });

    test('WS mismatch reconcile getTask TaskNotFoundException → taskDeleted', () async {
      stubList();
      var getN = 0;
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async {
        getN++;
        if (getN == 1) {
          return task(status: 'in_progress');
        }
        throw TaskNotFoundException('gone');
      });
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      ctrl.applyWsTaskStatus(
        WsTaskStatusEvent(
          ts: DateTime.utc(2026, 5, 1),
          v: 1,
          projectId: pid,
          taskId: tid,
          previousStatus: 'pending',
          status: 'paused',
        ),
      );

      await Future<void>.delayed(const Duration(milliseconds: 60));

      final v = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(v.taskDeleted, isTrue);
      expect(v.task, isNull);
    });

    test('applyWsTaskStatus mismatch: second during reconcile inflight dropped', () async {
      stubList();
      var getCalls = 0;
      final hang = Completer<TaskModel>();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async {
        getCalls++;
        if (getCalls == 1) {
          return task(status: 'in_progress');
        }
        if (getCalls == 2) {
          return hang.future;
        }
        return task(status: 'paused');
      });
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final ev = WsTaskStatusEvent(
        ts: DateTime.utc(2026, 6, 1),
        v: 1,
        projectId: pid,
        taskId: tid,
        previousStatus: 'pending',
        status: 'paused',
      );

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(getCalls, 2);

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(getCalls, 2);

      hang.complete(task(status: 'paused'));
      await Future<void>.delayed(const Duration(milliseconds: 60));

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 60));
      expect(getCalls, greaterThan(2));
    });
  });
}
