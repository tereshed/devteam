import 'package:freezed_annotation/freezed_annotation.dart';

part 'scout_config_model.freezed.dart';
part 'scout_config_model.g.dart';

/// Конфиг разведчика проекта. Зеркалит backend `dto.ScoutConfigResponse`.
@freezed
abstract class ScoutConfigModel with _$ScoutConfigModel {
  const factory ScoutConfigModel({
    @JsonKey(name: 'project_id') required String projectId,
    @JsonKey(name: 'is_enabled') @Default(false) bool isEnabled,
    @Default('') String prompt,
    @JsonKey(name: 'code_backend') @Default('claude-code') String codeBackend,
    @JsonKey(name: 'provider_kind') String? providerKind,
    double? temperature,
    @JsonKey(name: 'code_backend_settings')
    @Default(<String, dynamic>{})
    Map<String, dynamic> codeBackendSettings,
    @JsonKey(name: 'sandbox_permissions')
    @Default(<String, dynamic>{})
    Map<String, dynamic> sandboxPermissions,
    @JsonKey(name: 'subscription_id') String? subscriptionId,
    @JsonKey(name: 'timeout_seconds') @Default(600) int timeoutSeconds,
  }) = _ScoutConfigModel;

  factory ScoutConfigModel.fromJson(Map<String, dynamic> json) =>
      _$ScoutConfigModelFromJson(json);
}
