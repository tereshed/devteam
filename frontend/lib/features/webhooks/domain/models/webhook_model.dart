import 'package:freezed_annotation/freezed_annotation.dart';

part 'webhook_model.freezed.dart';
part 'webhook_model.g.dart';

@freezed
abstract class WebhookModel with _$WebhookModel {
  const factory WebhookModel({
    required String id,
    required String name,
    @JsonKey(name: 'project_id') String? projectId,
    @JsonKey(name: 'team_id') String? teamId,
    @Default('') String instructions,
    @Default('') String description,
    @JsonKey(name: 'webhook_url') @Default('') String webhookUrl,
    @JsonKey(name: 'allowed_ips') @Default('') String allowedIps,
    @JsonKey(name: 'require_secret') @Default(false) bool requireSecret,
    @JsonKey(name: 'task_title_template') @Default('') String taskTitleTemplate,
    @JsonKey(name: 'task_description_template') @Default('') String taskDescriptionTemplate,
    @JsonKey(name: 'task_priority_template') @Default('') String taskPriorityTemplate,
    String? secret,
    @JsonKey(name: 'trigger_count') @Default(0) int triggerCount,
    @JsonKey(name: 'last_triggered') DateTime? lastTriggered,
    @JsonKey(name: 'is_active') @Default(true) bool isActive,
    @JsonKey(name: 'created_at') required DateTime createdAt,
  }) = _WebhookModel;

  factory WebhookModel.fromJson(Map<String, dynamic> json) =>
      _$WebhookModelFromJson(json);
}

@freezed
abstract class CreateWebhookRequest with _$CreateWebhookRequest {
  const factory CreateWebhookRequest({
    required String name,
    @JsonKey(name: 'project_id') String? projectId,
    @JsonKey(name: 'team_id') String? teamId,
    @Default('') String instructions,
    @Default('') String description,
    @JsonKey(name: 'allowed_ips') @Default('') String allowedIps,
    @JsonKey(name: 'require_secret') @Default(false) bool requireSecret,
    @JsonKey(name: 'task_title_template') @Default('') String taskTitleTemplate,
    @JsonKey(name: 'task_description_template') @Default('') String taskDescriptionTemplate,
    @JsonKey(name: 'task_priority_template') @Default('') String taskPriorityTemplate,
  }) = _CreateWebhookRequest;

  factory CreateWebhookRequest.fromJson(Map<String, dynamic> json) =>
      _$CreateWebhookRequestFromJson(json);
}

@freezed
abstract class UpdateWebhookRequest with _$UpdateWebhookRequest {
  const factory UpdateWebhookRequest({
    @JsonKey(name: 'project_id') String? projectId,
    @JsonKey(name: 'team_id') String? teamId,
    String? instructions,
    String? description,
    @JsonKey(name: 'allowed_ips') String? allowedIps,
    @JsonKey(name: 'require_secret') bool? requireSecret,
    @JsonKey(name: 'is_active') bool? isActive,
    @JsonKey(name: 'regenerate_secret') bool? regenerateSecret,
    @JsonKey(name: 'task_title_template') String? taskTitleTemplate,
    @JsonKey(name: 'task_description_template') String? taskDescriptionTemplate,
    @JsonKey(name: 'task_priority_template') String? taskPriorityTemplate,
  }) = _UpdateWebhookRequest;

  factory UpdateWebhookRequest.fromJson(Map<String, dynamic> json) =>
      _$UpdateWebhookRequestFromJson(json);
}
