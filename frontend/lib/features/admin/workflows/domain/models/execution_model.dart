import 'package:freezed_annotation/freezed_annotation.dart';

part 'execution_model.freezed.dart';
part 'execution_model.g.dart';

@freezed
abstract class Execution with _$Execution {
  const factory Execution({
    required String id,
    @JsonKey(name: 'workflow_id') required String workflowId,
    required String status,
    @JsonKey(name: 'current_step_id') String? currentStepId,
    @JsonKey(name: 'input_data') String? inputData,
    @JsonKey(name: 'output_data') String? outputData,
    @JsonKey(name: 'step_count') required int stepCount,
    @JsonKey(name: 'created_at') required DateTime createdAt,
    @JsonKey(name: 'finished_at') DateTime? finishedAt,
    @JsonKey(name: 'error_message') String? errorMessage,
  }) = _Execution;

  factory Execution.fromJson(Map<String, dynamic> json) =>
      _$ExecutionFromJson(json);
}

@freezed
abstract class ExecutionStep with _$ExecutionStep {
  const factory ExecutionStep({
    required String id,
    @JsonKey(name: 'step_id') required String stepId,
    @JsonKey(name: 'agent_name') String? agentName,
    @JsonKey(name: 'input_context') String? inputContext,
    @JsonKey(name: 'output_content') String? outputContent,
    @JsonKey(name: 'duration_ms') required int durationMs,
    @JsonKey(name: 'tokens_used') required int tokensUsed,
    @JsonKey(name: 'created_at') required DateTime createdAt,
  }) = _ExecutionStep;

  factory ExecutionStep.fromJson(Map<String, dynamic> json) =>
      _$ExecutionStepFromJson(json);
}

@freezed
abstract class ExecutionListResponse with _$ExecutionListResponse {
  const factory ExecutionListResponse({
    required List<Execution> executions,
    required int total,
  }) = _ExecutionListResponse;

  factory ExecutionListResponse.fromJson(Map<String, dynamic> json) =>
      _$ExecutionListResponseFromJson(json);
}
