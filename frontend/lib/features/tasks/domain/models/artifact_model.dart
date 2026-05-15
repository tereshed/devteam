import 'package:flutter/foundation.dart';

/// Артефакт оркестрации v2 (см. `artifacts` table в backend).
@immutable
class Artifact {
  final String id;
  final String taskId;
  final String? parentId;
  final String producerAgent;
  final String kind; // plan | subtask_description | code_diff | merged_code | review | test_result | ...
  final String summary;
  final Map<String, dynamic>? content;
  final String status; // ready | superseded
  final int iteration;
  final DateTime createdAt;

  const Artifact({
    required this.id,
    required this.taskId,
    required this.producerAgent,
    required this.kind,
    required this.summary,
    required this.status,
    required this.iteration,
    required this.createdAt,
    this.parentId,
    this.content,
  });

  factory Artifact.fromJson(Map<String, dynamic> json) {
    return Artifact(
      id: json['id'] as String,
      taskId: json['task_id'] as String,
      parentId: json['parent_id'] as String?,
      producerAgent: json['producer_agent'] as String? ?? '',
      kind: json['kind'] as String,
      summary: json['summary'] as String? ?? '',
      content: json['content'] is Map<String, dynamic>
          ? json['content'] as Map<String, dynamic>
          : null,
      status: json['status'] as String? ?? 'ready',
      iteration: (json['iteration'] as num?)?.toInt() ?? 0,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  /// Зависимости подзадачи (если kind == subtask_description).
  List<String> get dependsOn {
    if (kind != 'subtask_description' || content == null) {
      return const [];
    }
    final raw = content!['depends_on'];
    if (raw is! List) {
      return const [];
    }
    return raw.whereType<String>().toList(growable: false);
  }

  /// Заголовок подзадачи (если kind == subtask_description).
  String? get subtaskTitle {
    if (content == null) {
      return null;
    }
    final t = content!['title'];
    return t is String && t.isNotEmpty ? t : null;
  }
}
