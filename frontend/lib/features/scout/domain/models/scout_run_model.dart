import 'package:freezed_annotation/freezed_annotation.dart';

part 'scout_run_model.freezed.dart';
part 'scout_run_model.g.dart';

/// Прогон разведчика. Зеркалит backend `dto.ScoutRunResponse`.
@freezed
abstract class ScoutRunModel with _$ScoutRunModel {
  const factory ScoutRunModel({
    required String id,
    @JsonKey(name: 'project_id') required String projectId,
    @Default('running') String status,
    @JsonKey(name: 'code_backend') @Default('claude-code') String codeBackend,
    @Default('') String problem,
    @Default('') String dossier,
    @Default('') String error,
    @JsonKey(name: 'sandbox_instance_id') @Default('') String sandboxInstanceId,
    @JsonKey(name: 'started_at') required DateTime startedAt,
    @JsonKey(name: 'finished_at') DateTime? finishedAt,
  }) = _ScoutRunModel;

  factory ScoutRunModel.fromJson(Map<String, dynamic> json) =>
      _$ScoutRunModelFromJson(json);
}
