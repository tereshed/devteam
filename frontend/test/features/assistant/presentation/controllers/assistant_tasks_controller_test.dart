// @dart=2.19
@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/data/assistant_repository.dart';
import 'package:frontend/features/assistant/domain/assistant_active_task_model.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_tasks_controller.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'assistant_tasks_controller_test.mocks.dart';

@GenerateNiceMocks([
  MockSpec<AssistantRepository>(),
  MockSpec<WebSocketService>(),
])
void main() {
  const userId = '11111111-1111-1111-1111-111111111111';
  const projectA = 'pa';
  const projectB = 'pb';

  late MockAssistantRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;
  late ProviderContainer container;

  AssistantActiveTaskModel task(
    String id, {
    String project = projectA,
    String name = 'A',
    String state = 'active',
    required DateTime updatedAt,
  }) =>
      AssistantActiveTaskModel(
        taskId: id,
        projectId: project,
        projectName: name,
        title: 'task-$id',
        state: state,
        updatedAt: updatedAt,
      );

  WsClientEvent updateEv({
    required String taskId,
    required String state,
    required DateTime updatedAt,
    String project = projectA,
    String? title,
  }) =>
      WsClientEvent.server(WsServerEvent.assistantTaskUpdate(
        WsAssistantTaskUpdateEvent(
          ts: updatedAt,
          v: 1,
          userId: userId,
          projectId: project,
          taskId: taskId,
          state: state,
          title: title,
          updatedAt: updatedAt,
        ),
      ));

  AssistantTasksController ctrl() =>
      container.read(assistantTasksControllerProvider.notifier);

  AssistantTasksState st() =>
      container.read(assistantTasksControllerProvider);

  setUp(() {
    mockRepo = MockAssistantRepository();
    mockWs = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(mockWs.events).thenAnswer((_) => wsEvents.stream);
    container = ProviderContainer(
      overrides: [
        assistantRepositoryProvider.overrideWithValue(mockRepo),
        webSocketServiceProvider.overrideWithValue(mockWs),
      ],
    );
    addTearDown(() async {
      await wsEvents.close();
      container.dispose();
    });
  });

  group('AssistantTasksController.refresh', () {
    test('sorts tasks by updatedAt DESC and clears loading', () async {
      when(mockRepo.getActiveTasks(
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => AssistantActiveTasksResponse(
            tasks: [
              task('t-old', updatedAt: DateTime.utc(2026, 1, 1)),
              task('t-new', updatedAt: DateTime.utc(2026, 1, 3)),
              task('t-mid', updatedAt: DateTime.utc(2026, 1, 2)),
            ],
          ));

      await ctrl().refresh();

      expect(st().loading, isFalse);
      expect(st().tasks.map((t) => t.taskId).toList(),
          ['t-new', 't-mid', 't-old']);
    });

    test('REST error → loading=false, error stored, no rethrow', () async {
      final ex = AssistantApiException('500', statusCode: 500);
      when(mockRepo.getActiveTasks(
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(ex);

      await ctrl().refresh();

      expect(st().loading, isFalse);
      expect(st().error, equals(ex));
      expect(st().tasks, isEmpty);
    });

    test('clearError wipes only error', () async {
      when(mockRepo.getActiveTasks(
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(AssistantApiException('500', statusCode: 500));
      await ctrl().refresh();
      expect(st().error, isNotNull);

      ctrl().clearError();
      expect(st().error, isNull);
    });
  });

  group('AssistantTasksController WS', () {
    test('task_update inserts unknown task at top', () async {
      ctrl(); // force build() → подписаться на WS stream до отправки события.
      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'active',
        updatedAt: DateTime.utc(2026, 1, 3),
        title: 'New task',
      ));
      await _drain();

      expect(st().tasks.length, 1);
      final t = st().tasks.first;
      expect(t.taskId, 't1');
      expect(t.title, 'New task');
      // projectName неизвестен (нет REST snapshot'а) — пустая строка, UI
      // должен показать "—".
      expect(t.projectName, isEmpty);
    });

    test('task_update merges with existing REST snapshot (project_name kept)',
        () async {
      when(mockRepo.getActiveTasks(
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => AssistantActiveTasksResponse(
            tasks: [
              task('t1',
                  name: 'Project Alpha',
                  state: 'active',
                  updatedAt: DateTime.utc(2026, 1, 1)),
            ],
          ));
      await ctrl().refresh();

      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'done',
        updatedAt: DateTime.utc(2026, 1, 3),
      ));
      await _drain();

      expect(st().tasks.length, 1);
      final t = st().tasks.first;
      expect(t.state, 'done');
      expect(t.projectName, 'Project Alpha');
    });

    test('out-of-order updateAt ignored (defensive guard)', () async {
      ctrl();
      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'active',
        updatedAt: DateTime.utc(2026, 1, 3),
      ));
      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'done',
        updatedAt: DateTime.utc(2026, 1, 1), // старее
      ));
      await _drain();

      expect(st().tasks.first.state, 'active');
    });

    test('keeps tasks sorted by updatedAt DESC after merging WS update',
        () async {
      when(mockRepo.getActiveTasks(
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => AssistantActiveTasksResponse(
            tasks: [
              task('t1', updatedAt: DateTime.utc(2026, 1, 1)),
              task('t2',
                  project: projectB,
                  name: 'B',
                  updatedAt: DateTime.utc(2026, 1, 2)),
            ],
          ));
      await ctrl().refresh();
      expect(st().tasks.map((t) => t.taskId).toList(), ['t2', 't1']);

      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'active',
        updatedAt: DateTime.utc(2026, 1, 5), // самый свежий
      ));
      await _drain();

      expect(st().tasks.map((t) => t.taskId).toList(), ['t1', 't2']);
    });

    test('reset clears tasks list', () async {
      ctrl();
      wsEvents.add(updateEv(
        taskId: 't1',
        state: 'active',
        updatedAt: DateTime.utc(2026, 1, 3),
      ));
      await _drain();
      expect(st().tasks, isNotEmpty);

      ctrl().reset();
      expect(st().tasks, isEmpty);
    });
  });
}

Future<void> _drain() async {
  for (var i = 0; i < 5; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}
