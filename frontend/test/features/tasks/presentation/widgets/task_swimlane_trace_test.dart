import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/projects/domain/models/team_model.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/domain/models/task_event_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_swimlane_trace.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  const projectId = 'p1';
  const taskId = 't1';

  AgentModel agent(String name, String role) =>
      AgentModel(id: name, name: name, role: role, isActive: true);

  RouterDecision dec(int step, List<String> agents, DateTime t,
          {String? outcome}) =>
      RouterDecision(
        id: 'd$step',
        taskId: taskId,
        stepNo: step,
        chosenAgents: agents,
        reason: 'reason $step',
        createdAt: t,
        outcome: outcome,
      );

  Artifact art(String id, String producer, String kind, DateTime t,
          {String? parentId}) =>
      Artifact(
        id: id,
        taskId: taskId,
        producerAgent: producer,
        kind: kind,
        summary: '$kind summary',
        status: 'ready',
        iteration: 0,
        createdAt: t,
        parentId: parentId,
      );

  TeamModel team(List<AgentModel> agents) => TeamModel(
        id: 'team1',
        name: 'Team',
        projectId: projectId,
        type: 'development',
        agents: agents,
        createdAt: DateTime.utc(2026, 5, 30),
        updatedAt: DateTime.utc(2026, 5, 30),
      );

  Widget harness({
    required List<RouterDecision> decisions,
    required List<Artifact> artifacts,
    required List<AgentModel> agents,
    String taskState = 'active',
  }) {
    return ProviderScope(
      overrides: [
        teamProvider(projectId).overrideWith((ref) => team(agents)),
        taskRouterDecisionsProvider(taskId).overrideWith((ref) => decisions),
        taskArtifactsProvider(taskId).overrideWith((ref) => artifacts),
        taskEventsProvider(taskId).overrideWith((ref) => const <TaskEventModel>[]),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('en'),
        home: Scaffold(
          body: TaskSwimlaneTrace(
            projectId: projectId,
            taskId: taskId,
            taskState: taskState,
            onAgentSelected: (_) {},
          ),
        ),
      ),
    );
  }

  testWidgets('рендерит трейс с дорожками, легендой и без исключений',
      (tester) async {
    final t0 = DateTime.utc(2026, 5, 30, 10, 0, 0);
    await tester.pumpWidget(harness(
      agents: [agent('planner', 'planner'), agent('developer', 'developer')],
      decisions: [
        dec(0, ['planner'], t0),
        dec(1, ['developer'], t0.add(const Duration(minutes: 1))),
      ],
      artifacts: [
        art('a0', 'planner', 'plan', t0.add(const Duration(seconds: 30))),
        art('a1', 'developer', 'code_diff',
            t0.add(const Duration(minutes: 1, seconds: 40)),
            parentId: 'a0'),
      ],
    ));
    await tester.pump();

    // Легенда отрисована реальными Text-виджетами (англ. локаль).
    expect(find.text('router decision'), findsOneWidget);
    expect(find.text('dependency'), findsOneWidget);
    // Клик-оверлеи спанов присутствуют (InkWell поверх CustomPaint).
    expect(find.byType(InkWell), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  testWidgets('ревью changes_requested рендерится без исключений (янтарь)',
      (tester) async {
    final t0 = DateTime.utc(2026, 5, 30, 10, 0, 0);
    await tester.pumpWidget(harness(
      agents: [agent('reviewer', 'reviewer'), agent('merger', 'merger')],
      decisions: [
        dec(0, ['planner'], t0),
        dec(1, ['reviewer'], t0.add(const Duration(minutes: 1))),
        dec(2, ['merger'], t0.add(const Duration(minutes: 3))),
      ],
      artifacts: [
        Artifact(
          id: 'rev1',
          taskId: taskId,
          producerAgent: 'reviewer',
          kind: 'review',
          summary: 'changes_requested: missing CSRF state check',
          status: 'ready',
          iteration: 0,
          createdAt: t0.add(const Duration(minutes: 1, seconds: 20)),
        ),
      ],
    ));
    await tester.pump();

    // Легенда содержит пункт «нужны правки», рендер без падений.
    expect(find.text('Changes requested'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('длинный флоу прокручивается по горизонтали, а не сжимается',
      (tester) async {
    final t0 = DateTime.utc(2026, 5, 30, 10, 0, 0);
    // Много шагов в узком вьюпорте: лента шире экрана → должен включиться
    // горизонтальный скролл (регрессия: раньше всё сжималось/уезжало за экран).
    final decisions = [
      for (var i = 0; i < 30; i++)
        dec(i, [i.isEven ? 'developer' : 'reviewer'],
            t0.add(Duration(minutes: i))),
    ];
    await tester.pumpWidget(harness(
      agents: [agent('developer', 'developer'), agent('reviewer', 'reviewer')],
      decisions: decisions,
      artifacts: const [],
    ));
    await tester.pump();

    final horizontalScroll = find.byWidgetPredicate(
      (w) => w is SingleChildScrollView && w.scrollDirection == Axis.horizontal,
    );
    expect(horizontalScroll, findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('показывает пустое состояние без решений роутера',
      (tester) async {
    await tester.pumpWidget(harness(
      agents: const [],
      decisions: const [],
      artifacts: const [],
    ));
    await tester.pump();

    expect(find.textContaining('Waiting for the first router decision'),
        findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
