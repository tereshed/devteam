import 'package:freezed_annotation/freezed_annotation.dart';

part 'scheduled_task_model.freezed.dart';
part 'scheduled_task_model.g.dart';

/// Регулярная (cron) задача проекта. Зеркалит backend `dto.ScheduledTaskResponse`.
@freezed
abstract class ScheduledTaskModel with _$ScheduledTaskModel {
  const factory ScheduledTaskModel({
    required String id,
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'team_id') String? teamId,
    @JsonKey(name: 'created_by') required String createdBy,
    required String name,
    @Default('') String description,
    @JsonKey(name: 'cron_expression') required String cronExpression,
    @Default('medium') String priority,
    @JsonKey(name: 'is_active') @Default(true) bool isActive,
    @JsonKey(name: 'last_run_at') DateTime? lastRunAt,
    @JsonKey(name: 'next_run_at') DateTime? nextRunAt,
    @JsonKey(name: 'created_at') required DateTime createdAt,
    @JsonKey(name: 'updated_at') required DateTime updatedAt,
  }) = _ScheduledTaskModel;

  factory ScheduledTaskModel.fromJson(Map<String, dynamic> json) =>
      _$ScheduledTaskModelFromJson(json);
}
