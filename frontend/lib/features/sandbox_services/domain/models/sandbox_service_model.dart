import 'package:freezed_annotation/freezed_annotation.dart';

part 'sandbox_service_model.freezed.dart';
part 'sandbox_service_model.g.dart';

/// Декларация эфемерного сервис-сайдкара проекта. Зеркалит backend
/// `dto.SandboxServiceConfigResponse` (Sprint 22).
@freezed
abstract class SandboxServiceModel with _$SandboxServiceModel {
  const factory SandboxServiceModel({
    @Default('') String id,
    @JsonKey(name: 'project_id') @Default('') String projectId,
    @JsonKey(name: 'is_enabled') @Default(false) bool isEnabled,
    @Default('postgres') String kind,
    @Default('db') String alias,
    @Default('postgres:16-alpine') String image,
    @JsonKey(name: 'db_name') @Default('app') String dbName,
    @JsonKey(name: 'db_user') @Default('postgres') String dbUser,
    @Default(5432) int port,
    @JsonKey(name: 'seed_kind') @Default('none') String seedKind,
    @JsonKey(name: 'seed_value') @Default('') String seedValue,
    @JsonKey(name: 'ready_timeout_seconds') @Default(60) int readyTimeoutSeconds,
  }) = _SandboxServiceModel;

  factory SandboxServiceModel.fromJson(Map<String, dynamic> json) =>
      _$SandboxServiceModelFromJson(json);
}
