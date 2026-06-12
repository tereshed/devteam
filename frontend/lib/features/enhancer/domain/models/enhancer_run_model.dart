import 'package:freezed_annotation/freezed_annotation.dart';

part 'enhancer_run_model.freezed.dart';
part 'enhancer_run_model.g.dart';

/// Прогон энхансера. Зеркалит backend `dto.EnhancerRunResponse`.
@freezed
abstract class EnhancerRunModel with _$EnhancerRunModel {
  const factory EnhancerRunModel({
    required String id,
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'trigger_kind') @Default('manual') String triggerKind,
    @Default('running') String status,
    @Default('') String report,
    @Default('') String error,
    @JsonKey(name: 'started_at') required DateTime startedAt,
    @JsonKey(name: 'finished_at') DateTime? finishedAt,
  }) = _EnhancerRunModel;

  factory EnhancerRunModel.fromJson(Map<String, dynamic> json) =>
      _$EnhancerRunModelFromJson(json);
}
