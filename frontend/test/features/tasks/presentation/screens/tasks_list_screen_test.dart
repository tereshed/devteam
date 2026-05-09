@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/screens/tasks_list_screen.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../../projects/helpers/project_dashboard_test_router.dart';
import '../../../projects/helpers/project_fixtures.dart';
import '../../../projects/helpers/test_wrappers.dart';
import '../../helpers/task_fixtures.dart';

/// Заглушки контроллера: не вызывают реальный [TaskListController.build] с валидацией UUID;
/// проверка невалидного `projectId` — в `task_list_controller_test.dart` (12.3).
///
/// **Family:** `taskListControllerProvider.overrideWith(...)` подменяет **все** инстансы
/// family для любого `projectId`. Для тестов здесь держим один провайдер на дерево — при
/// нескольких `projectId` в одном тесте нужен суженный override (12.5+).
class _StubTaskListController extends TaskListController {
  _StubTaskListController(this._seed);
  final TaskListState _seed;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;
}

class _TrackingRefreshTaskListController extends TaskListController {
  _TrackingRefreshTaskListController(this._seed);
  final TaskListState _seed;
  int refreshCalls = 0;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;

  @override
  Future<void> refresh({bool clearRealtimeBlocksOnSuccess = true}) async {
    refreshCalls++;
  }
}

class _TrackingLoadMoreTaskListController extends TaskListController {
  _TrackingLoadMoreTaskListController(this._seed);
  final TaskListState _seed;
  int loadMoreCalls = 0;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;

  @override
  Future<void> loadMore() async {
    loadMoreCalls++;
  }
}

class _ErrorBuildTaskListController extends TaskListController {
  @override
  FutureOr<TaskListState> build({required String projectId}) {
    throw StateError('task list failed');
  }
}

Widget _tasksScreenHarness({
  required List<Override> overrides,
  required Widget child,
}) {
  return ProviderScope(
    retry: (_, _) => null,
    overrides: overrides,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en'),
      home: Scaffold(body: child),
    ),
  );
}

void main() {
  group('TasksListScreen', () {
    testWidgets('router smoke: /projects/:id/tasks строит TasksListScreen', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTaskFixtureProjectId/tasks',
      );
      final seed = makeTaskListStateFixture();

      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            projectProvider(kTaskFixtureProjectId).overrideWith(
              (ref) async => makeProject(
                id: kTaskFixtureProjectId,
                name: kTestDashboardProjectNameFixtureAlpha,
              ),
            ),
            taskListControllerProvider.overrideWith(
              () => _StubTaskListController(seed),
            ),
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
      expect(find.byType(TasksListScreen), findsOneWidget);
    });

    testWidgets('empty без фильтров: tasksEmpty', (tester) async {
      final seed = makeTaskListStateFixture();
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(
              () => _StubTaskListController(seed),
            ),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(TasksListScreen)),
      )!;
      expect(find.text(l10n.tasksEmpty), findsOneWidget);
    });

    testWidgets('empty с фильтром: tasksEmptyFiltered + clear', (tester) async {
      final seed = makeTaskListStateFixture(
        filter: TaskListFilter.defaults().copyWith(search: 'z'),
      );
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(
              () => _StubTaskListController(seed),
            ),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(TasksListScreen)),
      )!;
      expect(find.text(l10n.tasksEmptyFiltered), findsOneWidget);
      expect(find.text(l10n.tasksEmptyFilteredClear), findsOneWidget);
    });

    testWidgets('ошибка первичной загрузки: иконка ошибки и retry', (
      tester,
    ) async {
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(
              _ErrorBuildTaskListController.new,
            ),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(TasksListScreen)),
      )!;
      expect(find.byIcon(Icons.error_outline), findsOneWidget);
      expect(find.text(l10n.retry), findsOneWidget);
    });

    testWidgets('narrow: pull-to-refresh вызывает notifier.refresh', (
      tester,
    ) async {
      useViewSize(tester, const Size(480, 800));
      final seed = makeTaskListStateFixture(
        items: [
          makeTaskListItemFixture(id: '11111111-1111-1111-1111-111111111111'),
        ],
        total: 1,
        offset: 1,
        hasMore: false,
      );
      late final _TrackingRefreshTaskListController ctrl;
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(() {
              ctrl = _TrackingRefreshTaskListController(seed);
              return ctrl;
            }),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      await tester.drag(
        find.byType(CustomScrollView).first,
        const Offset(0, 400),
      );
      await tester.pumpAndSettle();
      expect(ctrl.refreshCalls, greaterThan(0));
    });

    testWidgets('wide: IconButton.refresh вызывает refresh', (tester) async {
      useViewSize(tester, const Size(900, 800));
      final seed = makeTaskListStateFixture(
        items: [
          makeTaskListItemFixture(id: '22222222-2222-2222-2222-222222222222'),
        ],
        total: 1,
      );
      late final _TrackingRefreshTaskListController ctrl;
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(() {
              ctrl = _TrackingRefreshTaskListController(seed);
              return ctrl;
            }),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithIcon(IconButton, Icons.refresh));
      await tester.pumpAndSettle();
      expect(ctrl.refreshCalls, 1);
    });

    testWidgets('баннер loadMoreError и retry loadMore', (tester) async {
      useViewSize(tester, const Size(480, 800));
      final err = StateError('page');
      final seed = makeTaskListStateFixture(
        items: [
          makeTaskListItemFixture(id: '33333333-3333-3333-3333-333333333333'),
        ],
        total: 5,
        offset: 1,
        hasMore: true,
        loadMoreError: err,
      );
      late final _TrackingLoadMoreTaskListController ctrl;
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(() {
              ctrl = _TrackingLoadMoreTaskListController(seed);
              return ctrl;
            }),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(TasksListScreen)),
      )!;
      expect(find.text(l10n.retry), findsWidgets);
      await tester.tap(
        find.descendant(
          of: find.byKey(kTasksListLoadMoreErrorBannerKey),
          matching: find.text(l10n.retry),
        ),
      );
      await tester.pumpAndSettle();
      expect(ctrl.loadMoreCalls, 1);
    });

    testWidgets('тап по задаче: taskDetailNotImplementedYet', (tester) async {
      useViewSize(tester, const Size(480, 800));
      final seed = makeTaskListStateFixture(
        items: [
          makeTaskListItemFixture(
            id: '44444444-4444-4444-4444-444444444444',
            title: 'TapMeTitle',
          ),
        ],
        total: 1,
      );
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(
              () => _StubTaskListController(seed),
            ),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('TapMeTitle'));
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(TasksListScreen)),
      )!;
      expect(find.text(l10n.taskDetailNotImplementedYet), findsOneWidget);
    });

    testWidgets('wide + empty: без RefreshIndicator (п. 6 UI)', (tester) async {
      useViewSize(tester, const Size(900, 800));
      final seed = makeTaskListStateFixture();
      await tester.pumpWidget(
        _tasksScreenHarness(
          overrides: [
            taskListControllerProvider.overrideWith(
              () => _StubTaskListController(seed),
            ),
          ],
          child: const TasksListScreen(projectId: kTaskFixtureProjectId),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(RefreshIndicator), findsNothing);
    });
  });
}
