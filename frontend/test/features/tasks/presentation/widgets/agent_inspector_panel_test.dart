import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/agent_inspector_panel.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_execution_graph.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  Artifact art(String id, String kind, String summary) => Artifact(
        id: id,
        taskId: 't1',
        producerAgent: 'developer',
        kind: kind,
        summary: summary,
        status: 'ready',
        iteration: 1,
        createdAt: DateTime.utc(2026, 5, 30),
      );

  AgentNodeData node({
    required NodeStatus status,
    List<String> subtasks = const [],
    List<Artifact> artifacts = const [],
  }) =>
      AgentNodeData(
        id: 'n1',
        name: 'developer',
        role: 'developer',
        status: status,
        subtasks: subtasks,
        artifacts: artifacts,
        stepNo: 2,
      );

  Widget harness(AgentNodeData agent) => ProviderScope(
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          home: Scaffold(
            body: SizedBox(
              width: 360,
              child: AgentInspectorPanel(
                projectId: 'p1',
                taskId: 't1',
                agent: agent,
                onClose: () {},
              ),
            ),
          ),
        ),
      );

  testWidgets('компактный инспектор рендерит артефакты и мета без исключений',
      (tester) async {
    await tester.pumpWidget(harness(node(
      status: NodeStatus.success,
      subtasks: const ['Implement search'],
      artifacts: [art('a1', 'code_diff', '+84 -12'), art('a2', 'review', 'ok')],
    )));
    await tester.pump();

    expect(find.text('developer'), findsWidgets);
    expect(find.text('code_diff'), findsOneWidget);
    expect(find.text('Success'), findsOneWidget);
    expect(find.byType(ExpansionTile), findsOneWidget); // секция логов свёрнута
    expect(tester.takeException(), isNull);
  });

  testWidgets('для пустого агента показывает компактную подсказку',
      (tester) async {
    await tester.pumpWidget(harness(node(status: NodeStatus.pending)));
    await tester.pump();

    expect(find.textContaining('No artifacts'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
