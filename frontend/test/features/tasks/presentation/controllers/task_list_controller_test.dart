@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/domain/task_model_to_list_item.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/l10n/app_localizations_en.dart';
import 'package:mockito/mockito.dart';

import '../../../../support/task_list_test_helpers.dart';
import '../../helpers/task_mocks.mocks.dart';

void main() {
  const pid = '550e8400-e29b-41d4-a716-446655440000';
  const otherPid = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
  const tid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
  const uid = '33333333-3333-3333-3333-333333333333';

  late MockTaskRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;
  late ProviderContainer container;

  TaskListItemModel listItem(
    String id, {
    String status = 'pending',
    DateTime? createdAt,
    DateTime? updatedAt,
    AgentSummaryModel? assignedAgent,
  }) {
    final c = createdAt ?? DateTime.utc(2026, 1, 1);
    final u = updatedAt ?? DateTime.utc(2026, 1, 2);
    return TaskListItemModel(
      id: id,
      projectId: pid,
      title: 't',
      status: status,
      priority: 'medium',
      createdByType: 'user',
      createdById: uid,
      createdAt: c,
      updatedAt: u,
      assignedAgent: assignedAgent,
    );
  }

  TaskModel taskModel(
    String id, {
    String status = 'pending',
    AgentSummaryModel? assignedAgent,
    DateTime? updatedAt,
  }) {
    return TaskModel(
      id: id,
      projectId: pid,
      title: 't',
      description: '',
      status: status,
      priority: 'medium',
      createdByType: 'user',
      createdById: uid,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: updatedAt ?? DateTime.utc(2026, 1, 2),
      assignedAgent: assignedAgent,
    );
  }

  Future<void> waitUntil(
    bool Function() ok, {
    Duration timeout = const Duration(seconds: 3),
  }) async {
    final sw = Stopwatch()..start();
    while (!ok()) {
      if (sw.elapsed > timeout) {
        fail('waitUntil timeout');
      }
      await Future<void>.delayed(const Duration(milliseconds: 2));
    }
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

  group('TaskListController', () {
    test('invalid projectId → AsyncError ArgumentError', () {
      final c = ProviderContainer(
        overrides: [taskRepositoryProvider.overrideWithValue(mockRepo)],
      );
      addTearDown(c.dispose);
      final st = c.read(taskListControllerProvider(projectId: ''));
      expect(st.hasError, isTrue);
      expect(st.error, isA<ArgumentError>());
    });

    test('first page: offset, total, hasMore', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskListResponse(
          tasks: [listItem('a')],
          total: 2,
          limit: 50,
          offset: 0,
        ),
      );

      container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      final v = container.read(taskListControllerProvider(projectId: pid)).requireValue;
      expect(v.items, hasLength(1));
      expect(v.total, 2);
      expect(v.offset, 1);
      expect(v.hasMore, isTrue);
    });

    test('loadMore uses single inflight Future', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskListResponse(
          tasks: [listItem('a')],
          total: 2,
          limit: 50,
          offset: 0,
        ),
      );

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      final completer = Completer<TaskListResponse>();
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: 1,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) => completer.future);

      final f1 = ctrl.loadMore();
      final f2 = ctrl.loadMore();
      expect(identical(f1, f2), isTrue);

      completer.complete(
        TaskListResponse(
          tasks: [listItem('b')],
          total: 2,
          limit: 50,
          offset: 1,
        ),
      );
      await f1;

      final v = container.read(taskListControllerProvider(projectId: pid)).requireValue;
      expect(v.items.map((e) => e.id).toSet(), {'a', 'b'});
      expect(v.offset, 2);
      expect(v.hasMore, isFalse);
    });

    test('ProjectNotFoundException → AsyncError', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(ProjectNotFoundException('x'));

      container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      final st = container.read(taskListControllerProvider(projectId: pid));
      expect(st.hasError, isTrue);
      expect(st.error, isA<ProjectNotFoundException>());
    });

    test('TaskCancelledException is not treated as network in title helper', () {
      final l10n = AppLocalizationsEn();
      expect(taskListErrorTitle(l10n, TaskCancelledException('c')), l10n.errorRequestCancelled);
    });

    test('TaskApiException statusCode null → generic title not network', () {
      final l10n = AppLocalizationsEn();
      final err = TaskApiException(
        'weird',
        statusCode: null,
        isNetworkTransportError: false,
      );
      expect(taskListErrorTitle(l10n, err), l10n.taskErrorGeneric);
      expect(taskListErrorTitle(l10n, err), isNot(l10n.errorNetwork));
    });

    test('createTask when list AsyncError → notReady', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(ProjectNotFoundException('x'));

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      expect(container.read(taskListControllerProvider(projectId: pid)).hasError, isTrue);

      final outcome = await ctrl.createTask(const CreateTaskRequest(title: 'n'));
      expect(outcome, TaskMutationOutcome.notReady);
      verifyNever(
        mockRepo.createTask(any, any, cancelToken: anyNamed('cancelToken')),
      );
    });

    test('loadMore failure stores loadMoreError without AsyncError', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskListResponse(
          tasks: [listItem('a')],
          total: 2,
          limit: 50,
          offset: 0,
        ),
      );

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      final boom = TaskApiException('x', statusCode: 500);
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: 1,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenThrow(boom);

      await ctrl.loadMore();

      final asyncSt = container.read(taskListControllerProvider(projectId: pid));
      final v = asyncSt.requireValue;
      expect(v.loadMoreError, boom);
      expect(asyncSt.hasError, isFalse);
    });

    test('syncListFromHttpTask + active search → invalidate (repo called again)', () async {
      var callCount = 0;
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        callCount++;
        return TaskListResponse(
          tasks: [listItem('a')],
          total: 1,
          limit: 50,
          offset: 0,
        );
      });

      final listCtrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);
      expect(callCount, 1);

      await listCtrl.setFilter(TaskListFilter.defaults().copyWith(search: 'hello'));
      await waitTaskListControllerIdle(container, pid);
      expect(callCount, greaterThanOrEqualTo(2));

      final freshCallCount = callCount;
      listCtrl.syncListFromHttpTask(taskModel('a'));
      await waitUntil(() => callCount > freshCallCount);
    });

    test('createTask when realtimeMutationBlocked → blockedByRealtime', () async {
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

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      ctrl.applyRealtimeFailure(const WsServiceFailure.authExpired());

      final outcome = await ctrl.createTask(
        const CreateTaskRequest(title: 'n'),
      );
      expect(outcome, TaskMutationOutcome.blockedByRealtime);
      verifyNever(
        mockRepo.createTask(any, any, cancelToken: anyNamed('cancelToken')),
      );
    });

    test('applyWsTaskStatus mismatch schedules single row refetch under guard', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskListResponse(
          tasks: [listItem(tid, status: 'in_progress')],
          total: 1,
          limit: 50,
          offset: 0,
        ),
      );

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => taskModel(tid, status: 'paused'));

      ctrl.applyWsTaskStatus(
        WsTaskStatusEvent(
          ts: DateTime.utc(2026, 1, 3),
          v: 1,
          projectId: pid,
          taskId: tid,
          previousStatus: 'pending',
          status: 'paused',
        ),
      );
      await Future<void>.delayed(const Duration(milliseconds: 30));

      verify(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).called(1);
      final row = container.read(taskListControllerProvider(projectId: pid)).requireValue.items.single;
      expect(row.status, 'paused');
    });

    test('taskModelToListItem maps shared fields', () {
      final m = taskModel(tid);
      final item = taskModelToListItem(m);
      expect(item.id, m.id);
      expect(item.projectId, m.projectId);
      expect(item.title, m.title);
    });

    test('requestRestRefetch: second during inflight dropped; third after completes runs', () async {
      var calls = 0;
      final hang = Completer<TaskListResponse>();
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        calls++;
        if (calls == 1) {
          return TaskListResponse(
            tasks: [listItem('a')],
            total: 1,
            limit: 50,
            offset: 0,
          );
        }
        if (calls == 2) {
          return hang.future;
        }
        return TaskListResponse(
          tasks: [listItem('a')],
          total: 1,
          limit: 50,
          offset: 0,
        );
      });

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);
      expect(calls, 1);

      ctrl.requestRestRefetch();
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(calls, 2);

      ctrl.requestRestRefetch();
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(calls, 2);

      hang.complete(
        TaskListResponse(
          tasks: [listItem('a')],
          total: 1,
          limit: 50,
          offset: 0,
        ),
      );
      await waitTaskListControllerIdle(container, pid);

      ctrl.requestRestRefetch();
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(calls, 3);
    });

    test('applyWsTaskStatus mismatch: second during getTask inflight dropped; next after completes runs', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => TaskListResponse(
          tasks: [listItem(tid, status: 'in_progress')],
          total: 1,
          limit: 50,
          offset: 0,
        ),
      );

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      var getCalls = 0;
      final hang = Completer<TaskModel>();
      when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async {
        getCalls++;
        if (getCalls == 1) {
          return hang.future;
        }
        return taskModel(tid, status: 'paused');
      });

      final ev = WsTaskStatusEvent(
        ts: DateTime.utc(2026, 1, 3),
        v: 1,
        projectId: pid,
        taskId: tid,
        previousStatus: 'pending',
        status: 'paused',
      );

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(getCalls, 1);

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 15));
      expect(getCalls, 1);

      hang.complete(taskModel(tid, status: 'paused'));
      await Future<void>.delayed(const Duration(milliseconds: 50));

      ctrl.applyWsTaskStatus(ev);
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(getCalls, 2);
    });

    test('applyRealtimeFailure(transient) keeps realtimeMutationBlocked false', () async {
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

      final ctrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      ctrl.applyRealtimeFailure(const WsServiceFailure.transient());
      expect(
        container.read(taskListControllerProvider(projectId: pid)).requireValue.realtimeMutationBlocked,
        isFalse,
      );
    });

    test('syncListFromHttpTask: model fails active filter → row removed', () async {
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((invocation) async {
        final f = invocation.namedArguments[#filter] as TaskListFilter;
        if (f.status == 'in_progress') {
          return TaskListResponse(
            tasks: [listItem('x', status: 'in_progress')],
            total: 1,
            limit: 50,
            offset: 0,
          );
        }
        return TaskListResponse(
          tasks: [listItem('x')],
          total: 1,
          limit: 50,
          offset: 0,
        );
      });

      final listCtrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);

      await listCtrl.setFilter(TaskListFilter.defaults().copyWith(status: 'in_progress'));
      await waitTaskListControllerIdle(container, pid);

      expect(
        container.read(taskListControllerProvider(projectId: pid)).requireValue.items.single.id,
        'x',
      );

      listCtrl.syncListFromHttpTask(taskModel('x', status: 'pending'));
      expect(
        container.read(taskListControllerProvider(projectId: pid)).requireValue.items,
        isEmpty,
      );
    });

    test('syncListFromHttpTask: unknown id and total > items.length → refresh', () async {
      var calls = 0;
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        calls++;
        return TaskListResponse(
          tasks: [listItem('a')],
          total: 2,
          limit: 50,
          offset: 0,
        );
      });

      final listCtrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);
      expect(calls, 1);

      final before = calls;
      listCtrl.syncListFromHttpTask(taskModel('unknown'));
      await waitUntil(() => calls > before);
    });

    test('syncListFromHttpTask: unknown id and total == items.length → upsert without listTasks', () async {
      var calls = 0;
      when(
        mockRepo.listTasks(
          pid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        calls++;
        return TaskListResponse(
          tasks: [listItem('a')],
          total: 1,
          limit: 50,
          offset: 0,
        );
      });

      final listCtrl = container.read(taskListControllerProvider(projectId: pid).notifier);
      await waitTaskListControllerIdle(container, pid);
      final afterLoad = calls;

      listCtrl.syncListFromHttpTask(taskModel('b'));
      await Future<void>.delayed(const Duration(milliseconds: 25));

      expect(calls, afterLoad);
      final ids =
          container.read(taskListControllerProvider(projectId: pid)).requireValue.items.map((e) => e.id).toSet();
      expect(ids, {'a', 'b'});
    });

    group('12.9 WebSocket bridge', () {
      test('T1 taskStatus other projectId is no-op', () async {
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => TaskListResponse(
            tasks: [listItem(tid, status: 'pending')],
            total: 1,
            limit: 50,
            offset: 0,
          ),
        );

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        final before =
            container.read(taskListControllerProvider(projectId: pid)).requireValue;

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
            container.read(taskListControllerProvider(projectId: pid)).requireValue;
        expect(after.items.single.status, before.items.single.status);
        expect(after.items.single.id, before.items.single.id);
      });

      test('T2 taskStatus in sync: copyWith only, no getTask', () async {
        const agent = AgentSummaryModel(
          id: '77777777-7777-7777-7777-777777777777',
          name: 'a',
          role: 'developer',
        );
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => TaskListResponse(
            tasks: [
              listItem(tid, status: 'in_progress', assignedAgent: agent),
            ],
            total: 1,
            limit: 50,
            offset: 0,
          ),
        );

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

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

        verifyNever(
          mockRepo.getTask(any, cancelToken: anyNamed('cancelToken')),
        );
        expect(
          container.read(taskListControllerProvider(projectId: pid)).requireValue.items.single.status,
          'review',
        );
      });

      test('T2b agent change in WS triggers single getTask row refetch', () async {
        const agentA = AgentSummaryModel(
          id: '77777777-7777-7777-7777-777777777777',
          name: 'A',
          role: 'developer',
        );
        const agentB = AgentSummaryModel(
          id: '88888888-8888-8888-8888-888888888888',
          name: 'B',
          role: 'developer',
        );
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => TaskListResponse(
            tasks: [
              listItem(tid, status: 'in_progress', assignedAgent: agentA),
            ],
            total: 1,
            limit: 50,
            offset: 0,
          ),
        );

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).thenAnswer(
          (_) async => taskModel(
            tid,
            status: 'review',
            assignedAgent: agentB,
            updatedAt: DateTime.utc(2026, 1, 20),
          ),
        );

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
        await Future<void>.delayed(const Duration(milliseconds: 30));

        verify(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).called(1);
        final row =
            container.read(taskListControllerProvider(projectId: pid)).requireValue.items.single;
        expect(row.status, 'review');
        expect(row.assignedAgent?.id, agentB.id);
      });

      test('T3 taskStatus mismatch triggers refetch path via WS', () async {
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async => TaskListResponse(
            tasks: [listItem(tid, status: 'in_progress')],
            total: 1,
            limit: 50,
            offset: 0,
          ),
        );

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        when(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => taskModel(tid, status: 'paused'));

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
        await Future<void>.delayed(const Duration(milliseconds: 30));

        verify(mockRepo.getTask(tid, cancelToken: anyNamed('cancelToken'))).called(1);
        expect(
          container.read(taskListControllerProvider(projectId: pid)).requireValue.items.single.status,
          'paused',
        );
      });

      test('T6 five errors needsRestRefetch: one refresh on wave', () async {
        var calls = 0;
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async {
            calls++;
            return TaskListResponse(
              tasks: [listItem('a')],
              total: 1,
              limit: 50,
              offset: 0,
            );
          },
        );

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);
        expect(calls, 1);

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
        await waitTaskListControllerIdle(container, pid);
        expect(calls, 2);
      });

      test('T7 after dispose WS event does not trigger listTasks', () async {
        var calls = 0;
        when(
          mockRepo.listTasks(
            pid,
            filter: anyNamed('filter'),
            limit: anyNamed('limit'),
            offset: anyNamed('offset'),
            cancelToken: anyNamed('cancelToken'),
          ),
        ).thenAnswer(
          (_) async {
            calls++;
            return TaskListResponse(
              tasks: [listItem('a')],
              total: 1,
              limit: 50,
              offset: 0,
            );
          },
        );

        final localWs = MockWebSocketService();
        var cancelCount = 0;
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

        localContainer.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(localContainer, pid);
        expect(calls, 1);

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
        expect(calls, 1);
      });

      test('T8 parseError and unknown: no throw, state stable', () async {
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

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        final before =
            container.read(taskListControllerProvider(projectId: pid)).requireValue;

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
            container.read(taskListControllerProvider(projectId: pid)).requireValue;
        expect(identical(before, after), isTrue);
      });

      test('T9 serviceFailure transient sets realtimeServiceFailure', () async {
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

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        wsEvents.add(
          const WsClientEvent.serviceFailure(WsServiceFailure.transient()),
        );
        await Future<void>.delayed(Duration.zero);

        expect(
          container
              .read(taskListControllerProvider(projectId: pid))
              .requireValue
              .realtimeServiceFailure,
          const WsServiceFailure.transient(),
        );
      });

      test('T10 onDispose cancels subscription; never disconnect()', () async {
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

        localContainer.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(localContainer, pid);

        localContainer.dispose();

        verifyNever(localWs.disconnect());
      });

      test('T11 authFailure: authenticationLost + mutationBlocked', () async {
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

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        wsEvents.add(const WsClientEvent.authFailure(WsAuthFailure()));
        await Future<void>.delayed(Duration.zero);

        final v =
            container.read(taskListControllerProvider(projectId: pid)).requireValue;
        expect(v.realtimeSessionFailure, const RealtimeSessionFailure.authenticationLost());
        expect(v.realtimeMutationBlocked, isTrue);
        expect(v.realtimeServiceFailure, isNull);
      });

      test('T12 subprotocolMismatch clears transient service failure only', () async {
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

        container.read(taskListControllerProvider(projectId: pid).notifier);
        await waitTaskListControllerIdle(container, pid);

        container
            .read(taskListControllerProvider(projectId: pid).notifier)
            .applyRealtimeFailure(const WsServiceFailure.transient());
        expect(
          container.read(taskListControllerProvider(projectId: pid)).requireValue.realtimeServiceFailure,
          isNotNull,
        );

        wsEvents.add(
          const WsClientEvent.subprotocolMismatch(
            WsSubprotocolMismatch(expected: 'bearer.<jwt>'),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        final v =
            container.read(taskListControllerProvider(projectId: pid)).requireValue;
        expect(v.realtimeServiceFailure, isNull);
        expect(v.realtimeSessionFailure, isNull);
        expect(v.realtimeMutationBlocked, isFalse);
      });

      test(
        'T13 connect() StateError → transient via microtask before listTasks completes',
        () async {
          when(mockWs.connect(any)).thenThrow(StateError('paused'));
          addTearDown(() {
            when(mockWs.connect(any)).thenAnswer((_) => wsEvents.stream);
          });

          final hang = Completer<TaskListResponse>();
          when(
            mockRepo.listTasks(
              pid,
              filter: anyNamed('filter'),
              limit: anyNamed('limit'),
              offset: anyNamed('offset'),
              cancelToken: anyNamed('cancelToken'),
            ),
          ).thenAnswer((_) => hang.future);

          container.read(taskListControllerProvider(projectId: pid).notifier);

          await Future<void>.delayed(Duration.zero);
          await Future<void>.delayed(Duration.zero);

          expect(
            container
                .read(taskListControllerProvider(projectId: pid))
                .requireValue
                .realtimeServiceFailure,
            const WsServiceFailure.transient(),
          );

          hang.complete(
            const TaskListResponse(tasks: [], total: 0, limit: 50, offset: 0),
          );
          await waitTaskListControllerIdle(container, pid);
        },
      );
    });
  });
}
