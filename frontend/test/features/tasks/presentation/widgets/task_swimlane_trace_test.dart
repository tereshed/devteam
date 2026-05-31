import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/projects/domain/models/team_model.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
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
