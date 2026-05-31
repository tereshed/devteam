import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_execution_graph.dart';

void main() {
  AgentModel agent(String name, String role) =>
      AgentModel(id: name, name: name, role: role, isActive: true);

  RouterDecision dec(int step, List<String> agents, DateTime t, {String? outcome}) =>
      RouterDecision(
        id: 'd$step',
        taskId: 't',
        stepNo: step,
        chosenAgents: agents,
        reason: 'reason for step $step',
        createdAt: t,
        outcome: outcome,
      );

  group('buildAgentNodes — router & orchestrator nodes', () {
    test('добавляет orchestrator-корень и router-ноду на каждый шаг', () {
      final t0 = DateTime.utc(2026, 5, 30, 10, 0, 0);
      final decisions = [
        dec(0, ['planner'], t0),
        dec(1, ['developer'], t0.add(const Duration(minutes: 1))),
      ];
      final nodes = buildAgentNodes(
        decisions: decisions,
        artifacts: const [],
        taskState: 'active',
        assignedAgentName: null,
        assignedAgentRole: null,
        teamAgents: [agent('planner', 'planner'), agent('developer', 'developer')],
      );

      final orchestrators = nodes.where((n) => n.kind == NodeKind.orchestrator);
      final routers = nodes.where((n) => n.kind == NodeKind.router).toList();
      final agents = nodes.where((n) => n.kind == NodeKind.agent).toList();

      expect(orchestrators.length, 1, reason: 'ровно один корень-orchestrator');
      expect(routers.length, 2, reason: 'по router-ноде на каждое решение');
      expect(agents.map((n) => n.name), containsAll(['planner', 'developer']));
      // reason переносится в router-ноду
      expect(routers.first.reason, isNotNull);
      expect(routers.any((r) => r.reason == 'reason for step 0'), isTrue);
    });

    test('router-нода с outcome=needs_human помечается failed', () {
      final t0 = DateTime.utc(2026, 5, 30, 10, 0, 0);
      final nodes = buildAgentNodes(
        decisions: [dec(0, const [], t0, outcome: 'needs_human')],
        artifacts: const [],
        taskState: 'needs_human',
        assignedAgentName: null,
        assignedAgentRole: null,
        teamAgents: const [],
      );
      final router = nodes.firstWhere((n) => n.kind == NodeKind.router);
      expect(router.status, NodeStatus.failed);
    });
  });
}
