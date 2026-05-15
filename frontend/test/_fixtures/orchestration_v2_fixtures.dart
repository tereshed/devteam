// Fixture-builders для orchestration v2 моделей.
//
// Поскольку AgentV2 / Artifact / RouterDecision / WorktreeV2 написаны как
// `@immutable class`, а не freezed (см. CLAUDE.md task 6.7 — миграция на
// freezed отложена в отдельный PR), тестам достаточно простых билдеров
// с дефолтами для всех required-полей.

import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';

const String kFxTaskId = '11111111-1111-1111-1111-111111111111';
const String kFxAgentId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
const String kFxWorktreeId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';

DateTime _fxNow() => DateTime.utc(2026, 5, 15, 12);

AgentV2 fxAgent({
  String? id,
  String name = 'planner-claude',
  String role = 'planner',
  String roleDescription = 'Splits task into atomic subtasks.',
  String executionKind = 'llm',
  String? systemPrompt,
  String? model = 'claude-sonnet-4-6',
  double? temperature = 0.7,
  int? maxTokens = 4096,
  String? codeBackend,
  bool isActive = true,
}) {
  return AgentV2(
    id: id ?? kFxAgentId,
    name: name,
    role: role,
    roleDescription: roleDescription,
    executionKind: executionKind,
    systemPrompt: systemPrompt,
    model: executionKind == 'llm' ? model : null,
    temperature: executionKind == 'llm' ? temperature : null,
    maxTokens: executionKind == 'llm' ? maxTokens : null,
    codeBackend: executionKind == 'sandbox' ? (codeBackend ?? 'claude-code') : null,
    isActive: isActive,
    createdAt: _fxNow(),
    updatedAt: _fxNow(),
  );
}

AgentV2Page fxAgentPage(List<AgentV2> items, {int? limit, int? offset}) {
  return AgentV2Page(
    total: items.length,
    items: items,
    limit: limit ?? items.length,
    offset: offset ?? 0,
  );
}

WorktreeV2 fxWorktree({
  String? id,
  String? taskId,
  String state = 'in_use',
  String baseBranch = 'main',
  String? branchName,
  DateTime? allocatedAt,
  DateTime? releasedAt,
}) {
  return WorktreeV2(
    id: id ?? kFxWorktreeId,
    taskId: taskId ?? kFxTaskId,
    baseBranch: baseBranch,
    branchName: branchName ?? 'task-${(taskId ?? kFxTaskId).substring(0, 8)}-wt',
    state: state,
    allocatedAt: allocatedAt ?? _fxNow(),
    releasedAt: releasedAt,
  );
}

Artifact fxArtifact({
  required String kind,
  String? id,
  String? taskId,
  String producerAgent = 'developer',
  String summary = 'summary',
  String status = 'ready',
  int iteration = 1,
  Map<String, dynamic>? content,
  DateTime? createdAt,
}) {
  return Artifact(
    id: id ?? 'art-$kind-$iteration',
    taskId: taskId ?? kFxTaskId,
    producerAgent: producerAgent,
    kind: kind,
    summary: summary,
    status: status,
    iteration: iteration,
    content: content,
    createdAt: createdAt ?? _fxNow(),
  );
}

/// Удобный builder для subtask_description с depends_on.
Artifact fxSubtask({
  required String id,
  String title = 'Subtask',
  List<String> dependsOn = const [],
  String producerAgent = 'decomposer',
  int iteration = 1,
}) {
  return fxArtifact(
    id: id,
    kind: 'subtask_description',
    producerAgent: producerAgent,
    summary: title,
    iteration: iteration,
    content: <String, dynamic>{
      'title': title,
      'depends_on': dependsOn,
    },
  );
}

RouterDecision fxDecision({
  String? id,
  String? taskId,
  int stepNo = 1,
  List<String> agents = const ['planner'],
  String? outcome,
  String reason = 'route to planner',
  DateTime? createdAt,
}) {
  return RouterDecision(
    id: id ?? 'dec-$stepNo',
    taskId: taskId ?? kFxTaskId,
    stepNo: stepNo,
    chosenAgents: agents,
    outcome: outcome,
    reason: reason,
    createdAt: createdAt ?? _fxNow(),
  );
}
