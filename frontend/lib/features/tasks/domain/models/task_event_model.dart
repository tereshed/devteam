import 'package:flutter/foundation.dart';

@immutable
class TaskEventModel {
  final int id;
  final String taskId;
  final String kind;
  final Map<String, dynamic> payload;
  final DateTime createdAt;
  final DateTime? completedAt;

  const TaskEventModel({
    required this.id,
    required this.taskId,
    required this.kind,
    required this.payload,
    required this.createdAt,
    this.completedAt,
  });

  factory TaskEventModel.fromJson(Map<String, dynamic> json) {
    return TaskEventModel(
      id: (json['id'] as num).toInt(),
      taskId: json['task_id'] as String,
      kind: json['kind'] as String,
      payload: json['payload'] as Map<String, dynamic>? ?? const {},
      createdAt: DateTime.parse(json['created_at'] as String),
      completedAt: json['completed_at'] != null ? DateTime.parse(json['completed_at'] as String) : null,
    );
  }

  String get agentName => payload['agent'] as String? ?? '';
  
  Map<String, dynamic>? get inputData => payload['input'] as Map<String, dynamic>?;

  String? get targetArtifactId => inputData?['target_artifact_id'] as String?;

  List<String>? get targetArtifactIds {
    final raw = inputData?['target_artifact_ids'];
    if (raw is List) {
      return raw.cast<String>();
    }
    final single = targetArtifactId;
    if (single != null) {
      return [single];
    }
    return null;
  }
  
  String? get instructions => inputData?['instructions'] as String?;
}
