@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/domain/ws_task_message_mapper.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:mockito/mockito.dart';

import '../../../../support/task_list_test_helpers.dart';
import '../../helpers/task_mocks.mocks.dart';

void main() {
  const pid = '550e8400-e29b-41d4-a716-446655440000';
  const otherPid = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
  const tid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
  const otherTid = '11111111-1111-1111-1111-111111111111';
  const uid = '33333333-3333-3333-3333-333333333333';

  late MockTaskRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;
  late ProviderContainer container;

  TaskModel task({
    String status = 'pending',
    DateTime? updatedAt,
    List<TaskSummaryModel> subs = const [],
    String? projectIdOverride,
    String? errorMessage,
    AgentSummaryModel? assignedAgent,
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
      assignedAgent: assignedAgent,
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

  Future<void> waitDetail([ProviderContainer? c]) async {
    final ct = c ?? container;
    const step = Duration(milliseconds: 4);
    const timeout = Duration(seconds: 3);
    final sw = Stopwatch()..start();
    while (sw.elapsed < timeout) {
      final st = ct.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
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

  /// Только детальный контроллер (12.9): без второго подписчика на общий WS-stream.
  void listenDetailAlive() {
    container.listen(taskDetailControllerProvider(projectId: pid, taskId: tid), (_, __) {});
  }

  setUp(() {
    mockRepo = MockTaskRepository();
    mockWs = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(mockWs.events).thenAnswer((_) => wsEvents.stream);
    when(mockWs.connect(any)).thenAnswer((_) => wsEvents.stream);
    container = ProviderContainer(
      overrides: [
        taskRepositoryProvider.overrideWithValue(mockRepo),
        webSocketServiceProvider.overrideWithValue(mockWs),
      ],
    );
    addTearDown(() async {
      await wsEvents.close();
      container.dispose();
    });
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

    test('cancelTask sets lifecycleMutationInFlight until success then clears', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'active'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      final completer = Completer<TaskModel>();
      when(mockRepo.cancelTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) => completer.future);

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final before =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(before.lifecycleMutationInFlight, isNull);

      final fut = ctrl.cancelTask();
      final during =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(during.lifecycleMutationInFlight, TaskLifecycleMutation.cancel);

      completer.complete(task(status: 'cancelled', updatedAt: DateTime.utc(2026, 1, 3)));
      final o = await fut;
      expect(o, TaskMutationOutcome.completed);

      final after =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
      expect(after.lifecycleMutationInFlight, isNull);
      expect(after.task?.status, 'cancelled');
    });

    test('cancelTask repo throws → AsyncError с предыдущими данными (hasValue+hasError)', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'active'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );
      when(mockRepo.cancelTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(Exception('cancel failed'));

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      await expectLater(ctrl.cancelTask(), throwsException);

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.hasError, isTrue);
      expect(st.hasValue, isTrue);
      expect(st.requireValue.task?.status, 'active');
    });

    test('refresh: после успешной загрузки getTask падает — нет вечного loading ленты', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'planning'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );
      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(Exception('refresh failed'));

      await ctrl.refresh();

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.hasError, isTrue);
      expect(st.hasValue, isTrue);
      expect(st.requireValue.isLoadingTask, isFalse);
      expect(st.requireValue.isLoadingMessages, isFalse);
    });

    test('_patchState: правки доходят при AsyncError+copyWithPrevious', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'active'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );
      when(mockRepo.cancelTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(Exception('cancel failed'));

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      await expectLater(ctrl.cancelTask(), throwsException);

      expect(
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue
            .realtimeMutationBlocked,
        isFalse,
      );

      ctrl.setRealtimeMutationBlocked(true);

      final after = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(after.hasError, isTrue);
      expect(after.hasValue, isTrue);
      expect(after.requireValue.realtimeMutationBlocked, isTrue);
    });

    test('cancelTask in flight: refresh затем ошибка cancel — не AsyncError, без rethrow', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'active'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      final completer = Completer<TaskModel>();
      when(mockRepo.cancelTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) => completer.future);

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      final cancelFut = ctrl.cancelTask();
      await Future<void>.delayed(Duration.zero);

      await ctrl.refresh();

      completer.completeError(Exception('stale net'));
      final o = await cancelFut;
      expect(o, TaskMutationOutcome.completed);

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.hasError, isFalse);
      expect(st.requireValue.task?.status, 'active');
    });

    test('cancelTask race: 409 task_already_terminal → alreadyTerminal, без AsyncError', () async {
      stubList();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'active'));
      stubMessages(
        const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
      );

      listenKeepAlive();
      final ctrl =
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
      await waitDetail();

      // Первый вызов cancelTask: backend ответил 409 task_already_terminal.
      // После рекосиляции через _reloadTaskReconcileFromServer getTask вернёт уже-done.
      when(mockRepo.cancelTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(TaskAlreadyTerminalException('task is already in terminal state'));
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => task(status: 'done', updatedAt: DateTime.utc(2026, 1, 5)));

      final o = await ctrl.cancelTask();
      expect(o, TaskMutationOutcome.alreadyTerminal);

      // Дать reconcile-future отработать.
      await Future<void>.delayed(Duration.zero);

      final st = container.read(taskDetailControllerProvider(projectId: pid, taskId: tid));
      expect(st.hasError, isFalse,
          reason: 'race cancel should NOT escalate to AsyncError (no red snack)');
      expect(st.requireValue.task?.status, 'done',
          reason: 'controller must reconcile state from server after 409');
      expect(st.requireValue.lifecycleMutationInFlight, isNull);
    });

    test('cancelTask blockedByRealtime does not call repo', () async {
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
      final o = await ctrl.cancelTask();
      expect(o, TaskMutationOutcome.blockedByRealtime);
      verifyNever(mockRepo.cancelTask(any, cancelToken: anyNamed('cancelToken')));
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

    test('cancelTask when AsyncError → notReady', () async {
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

      final o = await ctrl.cancelTask();
      expect(o, TaskMutationOutcome.notReady);
      verifyNever(mockRepo.cancelTask(any, cancelToken: anyNamed('cancelToken')));
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

    group('12.9 WebSocket bridge', () {
      TaskMessageModel msg(String id, String content, DateTime createdAt) {
        return TaskMessageModel(
          id: id,
          taskId: tid,
          senderType: kSenderTypeAgent,
          senderId: uid,
          content: content,
          messageType: kMessageTypeResult,
          createdAt: createdAt,
        );
      }

      test('T1 taskStatus other projectId is no-op', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task(status: 'pending'));
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        final before =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskStatus(
              WsTaskStatusEvent(
                ts: DateTime.utc(2026, 1, 3),
                v: 1,
                projectId: otherPid,
                taskId: tid,
                previousStatus: 'pending',
                status: 'in_progress',
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final after =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
        expect(after.task?.status, before.task?.status);
        expect(after.task?.id, before.task?.id);
      });

      test('T2 taskStatus in sync: copyWith only, single getTask', () async {
        const agent = AgentSummaryModel(
          id: '77777777-7777-7777-7777-777777777777',
          name: 'a',
          role: 'developer',
        );
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task(status: 'in_progress', assignedAgent: agent));
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskStatus(
              WsTaskStatusEvent(
                ts: DateTime.utc(2026, 1, 3),
                v: 1,
                projectId: pid,
                taskId: tid,
                previousStatus: 'in_progress',
                status: 'review',
                assignedAgentId: agent.id,
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        verify(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).called(1);
        expect(
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task?.status,
          'review',
        );
      });

      test('T2b assignedAgentId change triggers reload getTask', () async {
        const agentA = AgentSummaryModel(
          id: '77777777-7777-7777-7777-777777777777',
          name: 'a',
          role: 'developer',
        );
        const agentB = AgentSummaryModel(
          id: '88888888-8888-8888-8888-888888888888',
          name: 'b',
          role: 'developer',
        );
        stubList();
        var getCalls = 0;
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          getCalls++;
          if (getCalls == 1) {
            return task(status: 'in_progress', assignedAgent: agentA);
          }
          return task(
            status: 'review',
            assignedAgent: agentB,
            updatedAt: DateTime.utc(2026, 1, 20),
          );
        });
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskStatus(
              WsTaskStatusEvent(
                ts: DateTime.utc(2026, 1, 3),
                v: 1,
                projectId: pid,
                taskId: tid,
                previousStatus: 'in_progress',
                status: 'review',
                assignedAgentId: agentB.id,
              ),
            ),
          ),
        );
        await Future<void>.delayed(const Duration(milliseconds: 50));

        expect(getCalls, 2);
        final t =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task;
        expect(t?.status, 'review');
        expect(t?.assignedAgent?.id, agentB.id);
      });

      test('T3 taskStatus mismatch triggers reconcile (second getTask)', () async {
        stubList();
        var getCalls = 0;
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          getCalls++;
          if (getCalls == 1) {
            return task(status: 'in_progress');
          }
          return task(status: 'paused', updatedAt: DateTime.utc(2026, 1, 10));
        });
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskStatus(
              WsTaskStatusEvent(
                ts: DateTime.utc(2026, 1, 3),
                v: 1,
                projectId: pid,
                taskId: tid,
                previousStatus: 'pending',
                status: 'paused',
              ),
            ),
          ),
        );
        await Future<void>.delayed(const Duration(milliseconds: 50));

        expect(getCalls, 2);
        expect(
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.task?.status,
          'paused',
        );
      });

      test('T3b reconcile TaskNotFound marks taskDeleted', () async {
        stubList();
        var getCalls = 0;
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          getCalls++;
          if (getCalls == 1) {
            return task(status: 'in_progress');
          }
          throw TaskNotFoundException('gone');
        });
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskStatus(
              WsTaskStatusEvent(
                ts: DateTime.utc(2026, 1, 3),
                v: 1,
                projectId: pid,
                taskId: tid,
                previousStatus: 'pending',
                status: 'paused',
              ),
            ),
          ),
        );
        await Future<void>.delayed(const Duration(milliseconds: 80));

        expect(getCalls, 2);
        final v =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
        expect(v.taskDeleted, isTrue);
        expect(v.task, isNull);
      });

      test('T4 taskMessage other taskId leaves messages list unchanged', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          TaskMessageListResponse(
            messages: [
              msg(
                'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
                'hello',
                DateTime.utc(2026, 1, 1),
              ),
            ],
            total: 1,
            limit: 50,
            offset: 0,
          ),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        final beforeIds = container
            .read(taskDetailControllerProvider(projectId: pid, taskId: tid))
            .requireValue
            .messages
            .map((m) => m.id)
            .toList();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskMessage(
              WsTaskMessageEvent(
                ts: DateTime.utc(2026, 1, 5),
                v: 1,
                projectId: pid,
                taskId: otherTid,
                messageId: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
                senderType: kSenderTypeAgent,
                senderId: uid,
                messageType: kMessageTypeResult,
                content: 'x',
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final afterIds = container
            .read(taskDetailControllerProvider(projectId: pid, taskId: tid))
            .requireValue
            .messages
            .map((m) => m.id)
            .toList();
        expect(afterIds, beforeIds);
      });

      test('T5 taskMessage for open task merges', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.taskMessage(
              WsTaskMessageEvent(
                ts: DateTime.utc(2026, 1, 6),
                v: 1,
                projectId: pid,
                taskId: tid,
                messageId: 'cccccccc-cccc-cccc-cccc-cccccccccccc',
                senderType: kSenderTypeAgent,
                senderId: uid,
                messageType: kMessageTypeResult,
                content: 'merged',
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final messages =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.messages;
        expect(messages.map((m) => m.id).toList(), ['cccccccc-cccc-cccc-cccc-cccccccccccc']);
        expect(messages.single.content, 'merged');
      });

      test('T6 five errors needsRestRefetch: single refresh wave', () async {
        stubList();
        // Второй getTask «висит»: пока refresh in-flight, дальнейшие WsErrorEvent
        // отбрасываются guard'ом `_wsRefetchInFlight` (антишторм).
        final hang = Completer<TaskModel>();
        var getTaskCalls = 0;
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          getTaskCalls++;
          if (getTaskCalls == 2) {
            return hang.future;
          }
          return task();
        });
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();
        expect(getTaskCalls, 1);

        for (var i = 0; i < 5; i++) {
          wsEvents.add(
            WsClientEvent.server(
              WsServerEvent.error(
                WsErrorEvent(
                  ts: DateTime.utc(2026, 1, 1),
                  v: i,
                  projectId: pid,
                  code: WsErrorCode.streamOverflow,
                  message: 'overflow',
                  needsRestRefetch: true,
                ),
              ),
            ),
          );
        }
        await Future<void>.delayed(Duration.zero);
        expect(getTaskCalls, 2);

        hang.complete(task());
        await waitDetail();
        expect(getTaskCalls, 2);

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.error(
              WsErrorEvent(
                ts: DateTime.utc(2026, 1, 2),
                v: 99,
                projectId: pid,
                code: WsErrorCode.streamOverflow,
                message: 'overflow',
                needsRestRefetch: true,
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);
        await waitDetail();
        expect(getTaskCalls, 3);
      });

      test('T7 after dispose: stream subscription cancelled, getTask not called again', () async {
        stubList();
        var getTaskCalls = 0;
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          getTaskCalls++;
          return task();
        });
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        var cancelCount = 0;
        final localWs = MockWebSocketService();
        final localStream = StreamController<WsClientEvent>.broadcast(
          onCancel: () => cancelCount++,
        );
        when(localWs.events).thenAnswer((_) => localStream.stream);
        when(localWs.connect(any)).thenAnswer((_) => localStream.stream);

        final localContainer = ProviderContainer(
          overrides: [
            taskRepositoryProvider.overrideWithValue(mockRepo),
            webSocketServiceProvider.overrideWithValue(localWs),
          ],
        );
        addTearDown(() async {
          await localStream.close();
        });

        localContainer.listen(taskDetailControllerProvider(projectId: pid, taskId: tid), (_, __) {});

        localContainer.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);

        await waitDetail(localContainer);
        expect(getTaskCalls, 1);

        localContainer.dispose();

        localStream.add(
          WsClientEvent.server(
            WsServerEvent.error(
              WsErrorEvent(
                ts: DateTime.utc(2026, 1, 1),
                v: 1,
                projectId: pid,
                code: WsErrorCode.streamOverflow,
                message: 'overflow',
                needsRestRefetch: true,
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        expect(cancelCount, 1);
        expect(getTaskCalls, 1);
      });

      test('T8 parseError and unknown: identical TaskDetailState instance', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        final before =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;

        wsEvents.add(const WsClientEvent.parseError(WsParseError(message: 'bad')));
        await Future<void>.delayed(Duration.zero);

        wsEvents.add(
          WsClientEvent.server(
            WsServerEvent.unknown(
              WsUnknownEvent(
                type: 'x',
                ts: DateTime.utc(2026, 1, 1),
                v: 1,
                projectId: pid,
                data: const <String, dynamic>{},
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final after =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
        expect(identical(before, after), isTrue);
      });

      test('T9 serviceFailure transient sets realtimeServiceFailure', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(
          const WsClientEvent.serviceFailure(WsServiceFailure.transient()),
        );
        await Future<void>.delayed(Duration.zero);

        expect(
          container
              .read(taskDetailControllerProvider(projectId: pid, taskId: tid))
              .requireValue
              .realtimeServiceFailure,
          const WsServiceFailure.transient(),
        );
      });

      test('T10 onDispose never disconnect()', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        final localWs = MockWebSocketService();
        final localStream = StreamController<WsClientEvent>.broadcast();
        when(localWs.events).thenAnswer((_) => localStream.stream);
        when(localWs.connect(any)).thenAnswer((_) => localStream.stream);

        final localContainer = ProviderContainer(
          overrides: [
            taskRepositoryProvider.overrideWithValue(mockRepo),
            webSocketServiceProvider.overrideWithValue(localWs),
          ],
        );
        addTearDown(() async {
          await localStream.close();
        });

        localContainer.listen(taskListControllerProvider(projectId: pid), (_, __) {});
        localContainer.listen(taskDetailControllerProvider(projectId: pid, taskId: tid), (_, __) {});

        localContainer.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);

        await waitDetail(localContainer);

        localContainer.dispose();

        verifyNever(localWs.disconnect());
      });

      test('T11 authFailure terminal session', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        wsEvents.add(const WsClientEvent.authFailure(WsAuthFailure()));
        await Future<void>.delayed(Duration.zero);

        final v =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
        expect(v.realtimeSessionFailure, const RealtimeSessionFailure.authenticationLost());
        expect(v.realtimeMutationBlocked, isTrue);
        expect(v.realtimeServiceFailure, isNull);
      });

      test('T12 subprotocolMismatch clears transient service failure only', () async {
        stubList();
        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => task());
        stubMessages(
          const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
        );

        listenDetailAlive();
        container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);
        await waitDetail();

        container
            .read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier)
            .applyRealtimeFailure(const WsServiceFailure.transient());
        expect(
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue.realtimeServiceFailure,
          isNotNull,
        );

        wsEvents.add(
          const WsClientEvent.subprotocolMismatch(
            WsSubprotocolMismatch(expected: 'bearer.<jwt>'),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final v =
            container.read(taskDetailControllerProvider(projectId: pid, taskId: tid)).requireValue;
        expect(v.realtimeServiceFailure, isNull);
        expect(v.realtimeSessionFailure, isNull);
        expect(v.realtimeMutationBlocked, isFalse);
      });

      test(
        'T13 connect() StateError → transient via microtask before getTask completes',
        () async {
          when(mockWs.connect(any)).thenThrow(StateError('paused'));
          addTearDown(() {
            when(mockWs.connect(any)).thenAnswer((_) => wsEvents.stream);
          });

          final hang = Completer<TaskModel>();
          when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
              .thenAnswer((_) => hang.future);
          stubMessages(
            const TaskMessageListResponse(messages: [], total: 0, limit: 50, offset: 0),
          );

          listenDetailAlive();
          container.read(taskDetailControllerProvider(projectId: pid, taskId: tid).notifier);

          await Future<void>.delayed(Duration.zero);
          await Future<void>.delayed(Duration.zero);

          expect(
            container
                .read(taskDetailControllerProvider(projectId: pid, taskId: tid))
                .requireValue
                .realtimeServiceFailure,
            const WsServiceFailure.transient(),
          );

          hang.complete(task());
          await waitDetail();
        },
      );
    });
  });
}
