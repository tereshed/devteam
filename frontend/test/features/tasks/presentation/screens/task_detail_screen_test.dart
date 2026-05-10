@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/domain/models/task_message_model.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/screens/task_detail_screen.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/l10n/app_localizations_en.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';
import 'package:frontend/shared/widgets/diff_viewer.dart';
import 'package:mockito/mockito.dart';

import '../../../projects/helpers/project_dashboard_test_router.dart';
import '../../../projects/helpers/project_fixtures.dart';
import '../../../projects/helpers/test_wrappers.dart';
import '../../helpers/task_fixtures.dart';
import '../../helpers/task_mocks.mocks.dart';

const String _kTid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
const String _kOtherProjectId = '44444444-4444-4444-4444-444444444444';
const String _kChildTid = '11111111-1111-1111-1111-111111111111';
const String _kMsgId = '22222222-2222-2222-2222-222222222222';

TaskModel _minimalTask({
  required String id,
  required String projectId,
  String title = 'Hello Task',
  String status = 'pending',
  List<TaskSummaryModel> subTasks = const [],
}) {
  return TaskModel(
    id: id,
    projectId: projectId,
    title: title,
    description: 'x',
    status: status,
    priority: 'medium',
    createdByType: 'user',
    createdById: kTaskFixtureUserId,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 2),
    subTasks: subTasks,
  );
}

/// Стаб [TaskDetailController]: фиксированный seed, опционально счётчики lifecycle / loadMore.
class _CountingDetailController extends TaskDetailController {
  _CountingDetailController(
    this._seed, {
    this.pauseResult = TaskMutationOutcome.completed,
    this.trackLoadMore = false,
  });

  final TaskDetailState _seed;
  final TaskMutationOutcome pauseResult;
  final bool trackLoadMore;

  int pauseCalls = 0;
  int cancelCalls = 0;
  int resumeCalls = 0;
  int loadMoreCalls = 0;

  @override
  FutureOr<TaskDetailState> build({
    required String projectId,
    required String taskId,
  }) =>
      _seed;

  @override
  Future<void> loadMoreMessages() async {
    if (trackLoadMore) {
      loadMoreCalls++;
      return;
    }
    await super.loadMoreMessages();
  }

  @override
  Future<void> retryMessagesAfterError() async {
    if (trackLoadMore) {
      loadMoreCalls++;
      return;
    }
    await super.retryMessagesAfterError();
  }

  @override
  Future<TaskMutationOutcome> pauseTask() async {
    pauseCalls++;
    return pauseResult;
  }

  @override
  Future<TaskMutationOutcome> cancelTask() async {
    cancelCalls++;
    return TaskMutationOutcome.completed;
  }

  @override
  Future<TaskMutationOutcome> resumeTask() async {
    resumeCalls++;
    return TaskMutationOutcome.completed;
  }
}

class _StubTaskListForDetail extends TaskListController {
  _StubTaskListForDetail(this._seed);
  final TaskListState _seed;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;
}

Future<void> _pumpDetail(
  WidgetTester tester, {
  List<Override> overrides = const [],
  TaskDetailController Function()? detailController,
  Size logicalSize = const Size(900, 800),
  Locale locale = const Locale('en'),
}) async {
  useViewSize(tester, logicalSize);
  final wsEvents = StreamController<WsClientEvent>.broadcast();
  addTearDown(wsEvents.close);
  final mockWs = MockWebSocketService();
  when(mockWs.events).thenAnswer((_) => wsEvents.stream);
  when(mockWs.connect(any)).thenAnswer((_) => wsEvents.stream);

  final built = <Override>[
    ...overrides,
    if (detailController != null)
      taskDetailControllerProvider(
        projectId: kTaskFixtureProjectId,
        taskId: _kTid,
      ).overrideWith(detailController),
    webSocketServiceProvider.overrideWithValue(mockWs),
  ];

  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: built,
      child: wrapSimple(
        const TaskDetailScreen(
          projectId: kTaskFixtureProjectId,
          taskId: _kTid,
        ),
        locale: locale,
      ),
    ),
  );
}

Finder _lifecycleButtonForLabel(String label) {
  const stripKey = ValueKey<String>('task_detail_lifecycle_mobile');
  final strip = find.byKey(stripKey, skipOffstage: false);
  final textInStrip = find.descendant(
    of: strip,
    matching: find.text(label),
    skipOffstage: false,
  );
  return find.ancestor(
    of: textInStrip,
    matching: find.bySubtype<ButtonStyleButton>(),
  );
}

void _expectProgressOnFilledLabel(
  WidgetTester tester, {
  required String onLabel,
  required String notOnLabel,
}) {
  final onF = _lifecycleButtonForLabel(onLabel);
  final offF = _lifecycleButtonForLabel(notOnLabel);
  expect(onF, findsOneWidget);
  expect(offF, findsOneWidget);
  expect(
    find.descendant(
      of: onF,
      matching: find.byType(CircularProgressIndicator),
    ),
    findsOneWidget,
  );
  expect(
    find.descendant(
      of: offF,
      matching: find.byType(CircularProgressIndicator),
    ),
    findsNothing,
  );
}

void main() {
  testWidgets('после загрузки показывает title задачи в AppBar', (tester) async {
    final mockRepo = MockTaskRepository();
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer(
      (_) async => _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, title: 'Hello Task'),
    );
    when(
      mockRepo.listTaskMessages(
        _kTid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => const TaskMessageListResponse(
        messages: [],
        total: 0,
        limit: 50,
        offset: 0,
      ),
    );

    await _pumpDetail(tester, overrides: [
      taskRepositoryProvider.overrideWithValue(mockRepo),
    ]);
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text('Hello Task'), findsOneWidget);
    expect(find.text(l10n.taskDetailAppBarLoading), findsNothing);
  });

  testWidgets(
    'narrow: успешная загрузка — RefreshIndicator, без refresh в AppBar',
    (tester) async {
      final mockRepo = MockTaskRepository();
      when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer(
        (_) async => _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, title: 'Narrow'),
      );
      when(
        mockRepo.listTaskMessages(
          _kTid,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );

      await _pumpDetail(
        tester,
        logicalSize: const Size(400, 800),
        overrides: [
          taskRepositoryProvider.overrideWithValue(mockRepo),
        ],
      );
      await tester.pumpAndSettle();

      expect(find.byType(RefreshIndicator), findsOneWidget);
      expect(find.byIcon(Icons.refresh), findsNothing);
      expect(find.text('Narrow'), findsWidgets);
    },
  );

  testWidgets('AsyncError: заголовок через l10n, без Exception.toString()', (
    tester,
  ) async {
    final mockRepo = MockTaskRepository();
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenThrow(TaskNotFoundException('gone'));

    await _pumpDetail(tester, overrides: [
      taskRepositoryProvider.overrideWithValue(mockRepo),
    ]);
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailErrorTaskNotFound), findsAtLeast(1));
    expect(find.textContaining('TaskNotFoundException'), findsNothing);
    expect(find.textContaining('gone'), findsNothing);
  });

  testWidgets('taskDeleted: AppBar и тело по l10n', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      taskDeleted: true,
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final stub = _CountingDetailController(seed);
    final l10n = AppLocalizationsEn();

    await _pumpDetail(tester, detailController: () => stub);
    await tester.pumpAndSettle();

    expect(find.text(l10n.taskDetailDeletedTitle), findsOneWidget);
    expect(find.text(l10n.taskDetailDeletedBody), findsOneWidget);
    expect(find.text(l10n.taskDetailBackToList), findsOneWidget);
    expect(find.byIcon(Icons.refresh), findsNothing);
  });

  testWidgets('taskDeleted: narrow — без RefreshIndicator и без refresh в AppBar', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      taskDeleted: true,
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final stub = _CountingDetailController(seed);

    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => stub,
    );
    await tester.pumpAndSettle();

    expect(find.byType(RefreshIndicator), findsNothing);
    expect(find.byIcon(Icons.refresh), findsNothing);
  });

  testWidgets(
    'несовпадение project_id: тело mismatch + к списку (ветка error:)',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
      final mockRepo = MockTaskRepository();
      when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer(
        (_) async => _minimalTask(id: _kTid, projectId: _kOtherProjectId),
      );

      await _pumpDetail(tester, overrides: [
        taskRepositoryProvider.overrideWithValue(mockRepo),
      ]);
      await tester.pumpAndSettle();
      final l10n = AppLocalizationsEn();

      expect(find.text(l10n.taskDetailProjectMismatch), findsWidgets);
      expect(find.text(l10n.taskDetailProjectMismatch), findsAtLeast(2));
      expect(find.text(l10n.taskDetailBackToList), findsOneWidget);
      expect(find.byIcon(Icons.refresh), findsNothing);
    },
  );

  testWidgets(
    'подзадача: tap → URL дочерней задачи; Back → /projects/:id/tasks',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
      final mockRepo = MockTaskRepository();
      final parentTask = _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        title: 'Parent task',
        subTasks: [
          const TaskSummaryModel(
            id: _kChildTid,
            title: 'Child subtask title',
            status: 'pending',
            priority: 'medium',
          ),
        ],
      );
      final childTask = _minimalTask(
        id: _kChildTid,
        projectId: kTaskFixtureProjectId,
        title: 'Child full',
      );

      when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => parentTask);
      when(mockRepo.getTask(_kChildTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => childTask);
      when(
        mockRepo.listTaskMessages(
          any,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );

      final listSeed = TaskListState(
        filter: TaskListFilter.defaults(),
        items: const [],
        total: 0,
        offset: 0,
        isLoadingInitial: false,
      );

      final wsEvents = StreamController<WsClientEvent>.broadcast();
      final mockWs = MockWebSocketService();
      when(mockWs.events).thenAnswer((_) => wsEvents.stream);
      when(mockWs.connect(any)).thenAnswer((_) => wsEvents.stream);
      addTearDown(wsEvents.close);

      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTaskFixtureProjectId/tasks/$_kTid',
      );

      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            projectProvider(kTaskFixtureProjectId).overrideWith(
              (ref) async => makeProject(id: kTaskFixtureProjectId, name: 'P'),
            ),
            taskListControllerProvider.overrideWith(
              () => _StubTaskListForDetail(listSeed),
            ),
            taskRepositoryProvider.overrideWithValue(mockRepo),
            webSocketServiceProvider.overrideWithValue(mockWs),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(router.state.uri.path, '/projects/$kTaskFixtureProjectId/tasks/$_kTid');

      await tester.tap(find.text('Child subtask title'));
      await tester.pumpAndSettle();

      expect(router.state.uri.path, '/projects/$kTaskFixtureProjectId/tasks/$_kChildTid');
      expect(find.text('Child full'), findsWidgets);

      await tester.tap(find.byType(BackButton));
      await tester.pumpAndSettle();

      expect(router.state.uri.path, '/projects/$kTaskFixtureProjectId/tasks');
    },
  );

  testWidgets(
    'messagesLoadMoreError: баннер и Retry вызывает loadMoreMessages',
    (tester) async {
      final msg = TaskMessageModel(
        id: _kMsgId,
        taskId: _kTid,
        senderType: kSenderTypeUser,
        senderId: kTaskFixtureUserId,
        content: 'Line',
        messageType: kMessageTypeInstruction,
        createdAt: DateTime.utc(2026, 1, 1),
      );
      final seed = TaskDetailState.initial().copyWith(
        task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId),
        isLoadingTask: false,
        isLoadingMessages: false,
        messages: [msg],
        messagesTotal: 2,
        messagesOffset: 1,
        hasMoreMessages: false,
        messagesLoadMoreError: Exception('pagination failed'),
      );
      final tracking = _CountingDetailController(seed, trackLoadMore: true);

      await _pumpDetail(tester, detailController: () => tracking);
      await tester.pumpAndSettle();

      expect(
        find.byKey(kTaskDetailMessagesLoadMoreErrorBannerKey),
        findsOneWidget,
      );

      await tester.tap(find.text('Retry'));
      await tester.pump();

      expect(tracking.loadMoreCalls, 1);
    },
  );

  testWidgets('realtimeMutationBlocked: баннер и lifecycle-кнопки отключены', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeMutationBlocked: true,
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailRealtimeMutationBlocked), findsOneWidget);
    final pause = tester.widget<IconButton>(
      find.ancestor(
        of: find.byTooltip(l10n.taskActionPause),
        matching: find.byType(IconButton),
      ),
    );
    final cancel = tester.widget<IconButton>(
      find.ancestor(
        of: find.byTooltip(l10n.taskActionCancel),
        matching: find.byType(IconButton),
      ),
    );
    expect(pause.onPressed, isNull);
    expect(cancel.onPressed, isNull);
  });

  testWidgets('realtimeSessionFailure: баннер', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeSessionFailure: const RealtimeSessionFailure.authenticationLost(),
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailRealtimeSessionFailure), findsOneWidget);
  });

  testWidgets('realtimeServiceFailure: баннер', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeServiceFailure: const WsServiceFailure.transient(),
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailRealtimeServiceFailure), findsOneWidget);
  });

  testWidgets('metadata: redacted JSON без sk- в тексте', (tester) async {
    final mockRepo = MockTaskRepository();
    final msg = TaskMessageModel(
      id: _kMsgId,
      taskId: _kTid,
      senderType: kSenderTypeUser,
      senderId: kTaskFixtureUserId,
      content: 'Body',
      messageType: kMessageTypeInstruction,
      metadata: {'batch_tag': 'job-99'},
      createdAt: DateTime.utc(2026, 1, 1),
    );
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer((_) async => _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId));
    when(
      mockRepo.listTaskMessages(
        _kTid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => TaskMessageListResponse(
        messages: [msg],
        total: 1,
        limit: 50,
        offset: 0,
      ),
    );

    await _pumpDetail(tester, overrides: [
      taskRepositoryProvider.overrideWithValue(mockRepo),
    ]);
    await tester.pumpAndSettle();

    expect(find.textContaining('batch_tag'), findsWidgets);
    expect(find.textContaining('sk-'), findsNothing);
  });

  testWidgets('12.8 planning wide: Pause и Cancel в AppBar, без Resume', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.byTooltip(l10n.taskActionPause), findsOneWidget);
    expect(find.byTooltip(l10n.taskActionCancel), findsOneWidget);
    expect(find.byTooltip(l10n.taskActionResume), findsNothing);
  });

  testWidgets('12.8 paused narrow: только Resume в теле', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'paused',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionResume), findsOneWidget);
    expect(find.text(l10n.taskActionPause), findsNothing);
    expect(find.text(l10n.taskActionCancel), findsNothing);
  });

  testWidgets('12.8 paused narrow: tap Resume вызывает resumeTask', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'paused',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionResume));
    await tester.pumpAndSettle();
    expect(tracking.resumeCalls, 1);
  });

  testWidgets('12.8 failed narrow: Resume как для paused', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'failed',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionResume), findsOneWidget);
    expect(find.text(l10n.taskActionPause), findsNothing);
  });

  testWidgets('12.8 completed: панель lifecycle отсутствует', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'completed',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionPause), findsNothing);
    expect(find.text(l10n.taskActionCancel), findsNothing);
    expect(find.text(l10n.taskActionResume), findsNothing);
  });

  testWidgets('12.8 неизвестный статус: панель lifecycle отсутствует', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'future_unknown_status',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionPause), findsNothing);
    expect(find.text(l10n.taskActionCancel), findsNothing);
    expect(find.text(l10n.taskActionResume), findsNothing);
  });

  testWidgets('12.8 pending narrow: только Cancel', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, status: 'pending'),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionCancel), findsOneWidget);
    expect(find.text(l10n.taskActionPause), findsNothing);
  });

  testWidgets('12.8 blockedByRealtime: SnackBar при Pause', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final stub = _CountingDetailController(
      seed,
      pauseResult: TaskMutationOutcome.blockedByRealtime,
    );
    await _pumpDetail(tester, detailController: () => stub);
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.byTooltip(l10n.taskActionPause));
    await tester.pumpAndSettle();
    expect(find.text(l10n.taskActionBlockedByRealtimeSnack), findsOneWidget);
  });

  testWidgets(
    '12.8 pause repo throws: SnackBar + Retry, карточка без полноэкранной ошибки',
    (tester) async {
      final mockRepo = MockTaskRepository();
      when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer(
        (_) async => _minimalTask(
          id: _kTid,
          projectId: kTaskFixtureProjectId,
          title: 'Hello Task',
          status: 'planning',
        ),
      );
      when(
        mockRepo.listTaskMessages(
          _kTid,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );
      when(mockRepo.pauseTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(Exception('pause failed'));

      await _pumpDetail(
        tester,
        logicalSize: const Size(400, 800),
        overrides: [
          taskRepositoryProvider.overrideWithValue(mockRepo),
        ],
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizationsEn();

      expect(find.text('Hello Task'), findsWidgets);
      expect(find.text(l10n.taskDetailBackToList), findsNothing);

      await tester.tap(find.text(l10n.taskActionPause));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 600));

      expect(find.text(l10n.taskErrorGeneric), findsOneWidget);
      expect(find.text(l10n.retry), findsOneWidget);
      expect(find.text('Hello Task'), findsWidgets);
      expect(find.text(l10n.taskDetailBackToList), findsNothing);
    },
  );

  testWidgets(
    '12.8 pause fail: WS-патчи с тем же error не дублируют snack',
    (tester) async {
      final mockRepo = MockTaskRepository();
      when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenAnswer(
        (_) async => _minimalTask(
          id: _kTid,
          projectId: kTaskFixtureProjectId,
          title: 'Hello Task',
          status: 'planning',
        ),
      );
      when(
        mockRepo.listTaskMessages(
          _kTid,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );
      when(mockRepo.pauseTask(_kTid, cancelToken: anyNamed('cancelToken')))
          .thenThrow(Exception('pause failed'));

      await _pumpDetail(
        tester,
        logicalSize: const Size(400, 800),
        overrides: [
          taskRepositoryProvider.overrideWithValue(mockRepo),
        ],
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizationsEn();

      await tester.tap(find.text(l10n.taskActionPause));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 600));

      expect(find.text(l10n.taskErrorGeneric), findsOneWidget);

      final container = ProviderScope.containerOf(
        tester.element(find.byType(TaskDetailScreen)),
      );
      final notifier = container.read(
        taskDetailControllerProvider(projectId: kTaskFixtureProjectId, taskId: _kTid).notifier,
      );

      notifier.applyWsTaskMessage(
        WsTaskMessageEvent(
          ts: DateTime.utc(2026, 5, 9),
          v: 1,
          projectId: kTaskFixtureProjectId,
          taskId: _kTid,
          messageId: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
          senderType: 'agent',
          senderId: kTaskFixtureUserId,
          messageType: 'instruction',
          content: 'ws',
        ),
      );
      await tester.pump();
      expect(find.text(l10n.taskErrorGeneric), findsOneWidget);

      notifier.applyRealtimeFailure(const WsServiceFailure.transient());
      await tester.pump();
      expect(find.text(l10n.taskErrorGeneric), findsOneWidget);
    },
  );

  testWidgets('12.8 отмена: dismiss диалога не вызывает cancelTask', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionCancel));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.cancel));
    await tester.pumpAndSettle();
    expect(tracking.cancelCalls, 0);
  });

  testWidgets('12.8 отмена: подтверждение вызывает cancelTask один раз', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionCancel));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.taskActionConfirm));
    await tester.pumpAndSettle();
    expect(tracking.cancelCalls, 1);
  });

  testWidgets('12.8 inflight pause: Cancel отключён', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
      lifecycleMutationInFlight: TaskLifecycleMutation.pause,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pump();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionCancel));
    await tester.pump();
    expect(tracking.cancelCalls, 0);
    expect(find.byType(AlertDialog), findsNothing);
  });

  testWidgets('12.8 inflight cancel: Pause отключён, индикатор на Cancel (narrow)', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
      lifecycleMutationInFlight: TaskLifecycleMutation.cancel,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pump();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionPause), findsOneWidget);
    expect(find.text(l10n.taskActionCancel), findsOneWidget);
    _expectProgressOnFilledLabel(
      tester,
      onLabel: l10n.taskActionCancel,
      notOnLabel: l10n.taskActionPause,
    );
    await tester.tap(find.text(l10n.taskActionPause));
    await tester.pump();
    expect(tracking.pauseCalls, 0);
  });

  testWidgets('12.8 inflight resume: кнопка отключена с индикатором (narrow, paused)', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'paused',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
      lifecycleMutationInFlight: TaskLifecycleMutation.resume,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pump();
    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskActionResume), findsOneWidget);
    final resumeF = _lifecycleButtonForLabel(l10n.taskActionResume);
    expect(resumeF, findsOneWidget);
    expect(
      find.descendant(
        of: resumeF,
        matching: find.byType(CircularProgressIndicator),
      ),
      findsOneWidget,
    );
    await tester.tap(find.text(l10n.taskActionResume));
    await tester.pump();
    expect(tracking.resumeCalls, 0);
  });

  testWidgets('12.8 inflight cancel wide: Pause отключён, индикатор на Cancel в AppBar', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
      isLoadingTask: false,
      isLoadingMessages: false,
      lifecycleMutationInFlight: TaskLifecycleMutation.cancel,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(tester, detailController: () => tracking);
    await tester.pump();
    final appBar = find.byType(AppBar);
    final pauseIconInBar = find.descendant(
      of: appBar,
      matching: find.byIcon(Icons.pause),
    );
    final pauseIconButton = find.ancestor(
      of: pauseIconInBar,
      matching: find.byType(IconButton),
    );
    expect(pauseIconInBar, findsOneWidget);
    expect(tester.widget<IconButton>(pauseIconButton).onPressed, isNull);
    expect(
      find.descendant(
        of: appBar,
        matching: find.byType(CircularProgressIndicator),
      ),
      findsOneWidget,
    );
    await tester.tap(pauseIconButton);
    await tester.pump();
    expect(tracking.pauseCalls, 0);
  });

  testWidgets('12.8 двойной tap Pause: один вызов API', (tester) async {
    final mockRepo = MockTaskRepository();
    var pauseCalls = 0;
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer(
      (_) async => _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'planning',
      ),
    );
    when(
      mockRepo.listTaskMessages(
        _kTid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => const TaskMessageListResponse(
        messages: [],
        total: 0,
        limit: 50,
        offset: 0,
      ),
    );
    when(mockRepo.pauseTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer((_) async {
      pauseCalls++;
      await Future<void>.delayed(const Duration(milliseconds: 500));
      return _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        status: 'paused',
      );
    });

    final listSeed = TaskListState(
      filter: TaskListFilter.defaults(),
      items: const [],
      total: 0,
      offset: 0,
      isLoadingInitial: false,
    );

    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      overrides: [
        taskListControllerProvider.overrideWith(
          () => _StubTaskListForDetail(listSeed),
        ),
        taskRepositoryProvider.overrideWithValue(mockRepo),
      ],
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionPause));
    await tester.pump(const Duration(milliseconds: 50));
    await tester.tap(find.text(l10n.taskActionPause));
    await tester.pump();
    expect(pauseCalls, 1);
    await tester.pump(const Duration(milliseconds: 600));
  });

  testWidgets('12.10 pending narrow: Cancel и подтверждение в диалоге', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, status: 'pending'),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(
      tester,
      logicalSize: const Size(400, 800),
      detailController: () => tracking,
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.text(l10n.taskActionCancel));
    await tester.pumpAndSettle();
    expect(find.text(l10n.taskActionCancelConfirmTitle), findsOneWidget);
    await tester.tap(find.text(l10n.taskActionConfirm));
    await tester.pumpAndSettle();
    expect(tracking.cancelCalls, 1);
  });

  testWidgets('12.10 applyWsTaskStatus на карточке из stub', (tester) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, status: 'planning'),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(TaskDetailScreen)),
    );
    container
        .read(
          taskDetailControllerProvider(projectId: kTaskFixtureProjectId, taskId: _kTid).notifier,
        )
        .applyWsTaskStatus(
          WsTaskStatusEvent(
            ts: DateTime.utc(2026, 2, 1),
            v: 2,
            projectId: kTaskFixtureProjectId,
            taskId: _kTid,
            previousStatus: 'planning',
            status: 'in_progress',
            parentTaskId: null,
            assignedAgentId: null,
            agentRole: null,
            errorMessage: null,
          ),
        );
    await tester.pump();
    expect(find.text(taskStatusLabel(l10n, 'in_progress')), findsWidgets);
  });

  testWidgets('12.10 applyWsTaskMessage добавляет содержимое в ленту', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, status: 'completed'),
      isLoadingTask: false,
      isLoadingMessages: false,
      messages: const [],
    );
    await _pumpDetail(
      tester,
      detailController: () => _CountingDetailController(seed),
    );
    await tester.pumpAndSettle();
    final container = ProviderScope.containerOf(
      tester.element(find.byType(TaskDetailScreen)),
    );
    container
        .read(
          taskDetailControllerProvider(projectId: kTaskFixtureProjectId, taskId: _kTid).notifier,
        )
        .applyWsTaskMessage(
          WsTaskMessageEvent(
            ts: DateTime.utc(2026, 1, 1, 12),
            v: 1,
            projectId: kTaskFixtureProjectId,
            taskId: _kTid,
            messageId: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
            senderType: 'agent',
            senderId: kTaskFixtureUserId,
            senderRole: 'developer',
            messageType: 'result',
            content: 'Fixture WS message positive',
            metadata: const <String, dynamic>{},
          ),
        );
    await tester.pump();
    expect(find.text('Fixture WS message positive'), findsOneWidget);
  });

  testWidgets('12.10 in_progress wide: Pause тап вызывает pauseTask', (
    tester,
  ) async {
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: kTaskFixtureProjectId, status: 'in_progress'),
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final tracking = _CountingDetailController(seed);
    await _pumpDetail(tester, detailController: () => tracking);
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsEn();
    await tester.tap(find.byTooltip(l10n.taskActionPause));
    await tester.pumpAndSettle();
    expect(tracking.pauseCalls, 1);
  });

  testWidgets('12.10 RU-smoke: секция описания при загрузке из repo', (
    tester,
  ) async {
    final mockRepo = MockTaskRepository();
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer(
      (_) async => _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
        title: 'RU title',
      ),
    );
    when(
      mockRepo.listTaskMessages(
        _kTid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => const TaskMessageListResponse(
        messages: [],
        total: 0,
        limit: 50,
        offset: 0,
      ),
    );
    await _pumpDetail(
      tester,
      overrides: [
        taskRepositoryProvider.overrideWithValue(mockRepo),
      ],
      locale: const Locale('ru'),
    );
    await tester.pumpAndSettle();
    final l10nRu = AppLocalizationsRu();
    expect(find.text(l10nRu.taskDetailSectionDescription), findsOneWidget);
  });

  testWidgets('12.10 непустой artifacts.diff показывает DiffViewer', (
    tester,
  ) async {
    final mockRepo = MockTaskRepository();
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer(
      (_) async => _minimalTask(
        id: _kTid,
        projectId: kTaskFixtureProjectId,
      ).copyWith(
        artifacts: {
          'diff': '--- a/x.txt\n+++ b/x.txt\n@@ -1 +1 @@\n-OLD\n+NEW\n',
        },
      ),
    );
    when(
      mockRepo.listTaskMessages(
        _kTid,
        messageType: anyNamed('messageType'),
        senderType: anyNamed('senderType'),
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => const TaskMessageListResponse(
        messages: [],
        total: 0,
        limit: 50,
        offset: 0,
      ),
    );
    await _pumpDetail(tester, overrides: [
      taskRepositoryProvider.overrideWithValue(mockRepo),
    ]);
    await tester.pumpAndSettle();
    expect(find.byType(DiffViewer), findsOneWidget);
  });
}
