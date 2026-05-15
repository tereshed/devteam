import 'package:flutter/foundation.dart';

@immutable
class WorktreeV2 {
  final String id;
  final String taskId;
  final String? subtaskId;
  final String baseBranch;
  final String branchName;
  final String state; // allocated | in_use | released
  final DateTime allocatedAt;
  final DateTime? releasedAt;

  const WorktreeV2({
    required this.id,
    required this.taskId,
    required this.baseBranch,
    required this.branchName,
    required this.state,
    required this.allocatedAt,
    this.subtaskId,
    this.releasedAt,
  });

  factory WorktreeV2.fromJson(Map<String, dynamic> json) {
    return WorktreeV2(
      id: json['id'] as String,
      taskId: json['task_id'] as String,
      subtaskId: json['subtask_id'] as String?,
      baseBranch: json['base_branch'] as String? ?? '',
      branchName: json['branch_name'] as String? ?? '',
      state: json['state'] as String,
      allocatedAt: DateTime.parse(json['allocated_at'] as String),
      releasedAt: json['released_at'] == null
          ? null
          : DateTime.tryParse(json['released_at'] as String),
    );
  }
}
