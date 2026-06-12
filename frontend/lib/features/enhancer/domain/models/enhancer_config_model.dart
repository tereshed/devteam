import 'package:freezed_annotation/freezed_annotation.dart';

part 'enhancer_config_model.freezed.dart';
part 'enhancer_config_model.g.dart';

/// Конфиг энхансера проекта. Зеркалит backend `dto.EnhancerConfigResponse`.
@freezed
abstract class EnhancerConfigModel with _$EnhancerConfigModel {
  const factory EnhancerConfigModel({
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'is_active') @Default(false) bool isActive,
    @Default('propose') String autonomy,
    @JsonKey(name: 'cron_expression') String? cronExpression,
    @JsonKey(name: 'analysis_window_days') @Default(7) int analysisWindowDays,
    @JsonKey(name: 'max_changes_per_run') @Default(5) int maxChangesPerRun,
    @JsonKey(name: 'last_run_at') DateTime? lastRunAt,
    @JsonKey(name: 'next_run_at') DateTime? nextRunAt,
  }) = _EnhancerConfigModel;

  factory EnhancerConfigModel.fromJson(Map<String, dynamic> json) =>
      _$EnhancerConfigModelFromJson(json);
}
