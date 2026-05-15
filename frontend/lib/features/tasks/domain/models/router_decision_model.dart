import 'package:flutter/foundation.dart';

/// Решение Router-LLM по задаче (см. `router_decisions` table в backend).
@immutable
class RouterDecision {
  final String id;
  final String taskId;
  final int stepNo;
  final List<String> chosenAgents;
  final String? outcome;
  final String reason;
  final DateTime createdAt;

  const RouterDecision({
    required this.id,
    required this.taskId,
    required this.stepNo,
    required this.chosenAgents,
    required this.reason,
    required this.createdAt,
    this.outcome,
  });

  factory RouterDecision.fromJson(Map<String, dynamic> json) {
    final agents = json['chosen_agents'];
    final list = agents is List
        ? agents.whereType<String>().toList(growable: false)
        : const <String>[];
    return RouterDecision(
      id: json['id'] as String,
      taskId: json['task_id'] as String,
      stepNo: (json['step_no'] as num?)?.toInt() ?? 0,
      chosenAgents: list,
      outcome: json['outcome'] as String?,
      reason: json['reason'] as String? ?? '',
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  bool get done => outcome != null && outcome!.isNotEmpty;
}
