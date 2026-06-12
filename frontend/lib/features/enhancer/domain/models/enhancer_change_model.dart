import 'package:freezed_annotation/freezed_annotation.dart';

part 'enhancer_change_model.freezed.dart';
part 'enhancer_change_model.g.dart';

/// Предложение изменения от энхансера. Зеркалит backend `dto.EnhancerChangeResponse`.
@freezed
abstract class EnhancerChangeModel with _$EnhancerChangeModel {
  const factory EnhancerChangeModel({
    required String id,
    @JsonKey(name: 'run_id') required String runId,
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'target_kind') required String targetKind,
    @JsonKey(name: 'target_agent_id') String? targetAgentId,
    @Default(<String, dynamic>{}) Map<String, dynamic> payload,
    @Default('') String reason,
    @JsonKey(name: 'expected_effect') @Default('') String expectedEffect,
    @Default('proposed') String status,
    @JsonKey(name: 'created_at') required DateTime createdAt,
  }) = _EnhancerChangeModel;

  factory EnhancerChangeModel.fromJson(Map<String, dynamic> json) =>
      _$EnhancerChangeModelFromJson(json);
}
