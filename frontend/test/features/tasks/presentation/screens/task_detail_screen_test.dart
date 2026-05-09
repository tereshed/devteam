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
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/domain/models/task_message_model.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/screens/task_detail_screen.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/l10n/app_localizations_en.dart';
import 'package:mockito/mockito.dart';

import '../../../projects/helpers/project_dashboard_test_router.dart';
import '../../../projects/helpers/project_fixtures.dart';
import '../../../projects/helpers/test_wrappers.dart';
import '../../helpers/task_fixtures.dart';
import '../controllers/task_list_controller_test.mocks.dart';

const String _kPid = '550e8400-e29b-41d4-a716-446655440000';
const String _kTid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';
const String _kOtherProjectId = '44444444-4444-4444-4444-444444444444';
const String _kChildTid = '11111111-1111-1111-1111-111111111111';
const String _kMsgId = '22222222-2222-2222-2222-222222222222';

TaskModel _minimalTask({
  required String id,
  required String projectId,
  String title = 'Hello Task',
  List<TaskSummaryModel> subTasks = const [],
}) {
  return TaskModel(
    id: id,
    projectId: projectId,
    title: title,
    description: 'x',
    status: 'pending',
    priority: 'medium',
    createdByType: 'user',
    createdById: kTaskFixtureUserId,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 2),
    subTasks: subTasks,
  );
}

/// Заглушка [TaskDetailController] без сети (12.5).
class _StubTaskDetailController extends TaskDetailController {
  _StubTaskDetailController(this._seed);
  final TaskDetailState _seed;

  @override
  FutureOr<TaskDetailState> build({
    required String projectId,
    required String taskId,
  }) =>
      _seed;
}

class _TrackingLoadMoreTaskDetailController extends TaskDetailController {
  _TrackingLoadMoreTaskDetailController(this._seed);
  final TaskDetailState _seed;
  int loadMoreCalls = 0;

  @override
  FutureOr<TaskDetailState> build({
    required String projectId,
    required String taskId,
  }) =>
      _seed;

  @override
  Future<void> loadMoreMessages() async {
    loadMoreCalls++;
  }

  @override
  Future<void> retryMessagesAfterError() async {
    loadMoreCalls++;
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
  required List<Override> overrides,
  Size logicalSize = const Size(900, 800),
}) async {
  useViewSize(tester, logicalSize);
  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: overrides,
      child: const MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: Locale('en'),
        home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
      ),
    ),
  );
}

void main() {
  testWidgets('после загрузки показывает title задачи в AppBar', (tester) async {
    final mockRepo = MockTaskRepository();
    when(mockRepo.getTask(_kTid, cancelToken: anyNamed('cancelToken')))
        .thenAnswer(
      (_) async => _minimalTask(id: _kTid, projectId: _kPid, title: 'Hello Task'),
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
        (_) async => _minimalTask(id: _kTid, projectId: _kPid, title: 'Narrow'),
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
    useViewSize(tester, const Size(900, 800));
    final seed = TaskDetailState.initial().copyWith(
      taskDeleted: true,
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final stub = _StubTaskDetailController(seed);
    final l10n = AppLocalizationsEn();

    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
              .overrideWith(() => stub),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: Locale('en'),
          home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(l10n.taskDetailDeletedTitle), findsOneWidget);
    expect(find.text(l10n.taskDetailDeletedBody), findsOneWidget);
    expect(find.text(l10n.taskDetailBackToList), findsOneWidget);
    expect(find.byIcon(Icons.refresh), findsNothing);
  });

  testWidgets('taskDeleted: narrow — без RefreshIndicator и без refresh в AppBar', (
    tester,
  ) async {
    useViewSize(tester, const Size(400, 800));
    final seed = TaskDetailState.initial().copyWith(
      taskDeleted: true,
      isLoadingTask: false,
      isLoadingMessages: false,
    );
    final stub = _StubTaskDetailController(seed);

    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
              .overrideWith(() => stub),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: Locale('en'),
          home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
        ),
      ),
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
        projectId: _kPid,
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
        projectId: _kPid,
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

      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$_kPid/tasks/$_kTid',
      );

      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            projectProvider(_kPid).overrideWith(
              (ref) async => makeProject(id: _kPid, name: 'P'),
            ),
            taskListControllerProvider.overrideWith(
              () => _StubTaskListForDetail(listSeed),
            ),
            taskRepositoryProvider.overrideWithValue(mockRepo),
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

      expect(router.state.uri.path, '/projects/$_kPid/tasks/$_kTid');

      await tester.tap(find.text('Child subtask title'));
      await tester.pumpAndSettle();

      expect(router.state.uri.path, '/projects/$_kPid/tasks/$_kChildTid');
      expect(find.text('Child full'), findsWidgets);

      await tester.tap(find.byType(BackButton));
      await tester.pumpAndSettle();

      expect(router.state.uri.path, '/projects/$_kPid/tasks');
    },
  );

  testWidgets(
    'messagesLoadMoreError: баннер и Retry вызывает loadMoreMessages',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
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
        task: _minimalTask(id: _kTid, projectId: _kPid),
        isLoadingTask: false,
        isLoadingMessages: false,
        messages: [msg],
        messagesTotal: 2,
        messagesOffset: 1,
        hasMoreMessages: false,
        messagesLoadMoreError: Exception('pagination failed'),
      );
      final tracking = _TrackingLoadMoreTaskDetailController(seed);

      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
                .overrideWith(() => tracking),
          ],
          child: const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: Locale('en'),
            home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
          ),
        ),
      );
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

  testWidgets('realtimeMutationBlocked: баннер', (tester) async {
    useViewSize(tester, const Size(900, 800));
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: _kPid),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeMutationBlocked: true,
    );
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
              .overrideWith(() => _StubTaskDetailController(seed)),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: Locale('en'),
          home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailRealtimeMutationBlocked), findsOneWidget);
  });

  testWidgets('realtimeSessionFailure: баннер', (tester) async {
    useViewSize(tester, const Size(900, 800));
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: _kPid),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeSessionFailure: const RealtimeSessionFailure.authenticationLost(),
    );
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
              .overrideWith(() => _StubTaskDetailController(seed)),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: Locale('en'),
          home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsEn();
    expect(find.text(l10n.taskDetailRealtimeSessionFailure), findsOneWidget);
  });

  testWidgets('realtimeServiceFailure: баннер', (tester) async {
    useViewSize(tester, const Size(900, 800));
    final seed = TaskDetailState.initial().copyWith(
      task: _minimalTask(id: _kTid, projectId: _kPid),
      isLoadingTask: false,
      isLoadingMessages: false,
      realtimeServiceFailure: const WsServiceFailure.transient(),
    );
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          taskDetailControllerProvider(projectId: _kPid, taskId: _kTid)
              .overrideWith(() => _StubTaskDetailController(seed)),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: Locale('en'),
          home: TaskDetailScreen(projectId: _kPid, taskId: _kTid),
        ),
      ),
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
        .thenAnswer((_) async => _minimalTask(id: _kTid, projectId: _kPid));
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
}
