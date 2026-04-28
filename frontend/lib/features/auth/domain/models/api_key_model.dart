import 'package:freezed_annotation/freezed_annotation.dart';

part 'api_key_model.freezed.dart';
part 'api_key_model.g.dart';

/// ApiKeyModel представляет API-ключ пользователя
@freezed
abstract class ApiKeyModel with _$ApiKeyModel {
  const factory ApiKeyModel({
    required String id,
    required String name,
    @JsonKey(name: 'key_prefix') required String keyPrefix,
    required String scopes,
    @JsonKey(name: 'expires_at') DateTime? expiresAt,
    @JsonKey(name: 'last_used_at') DateTime? lastUsedAt,
    @JsonKey(name: 'created_at') required DateTime createdAt,
  }) = _ApiKeyModel;

  const ApiKeyModel._();

  factory ApiKeyModel.fromJson(Map<String, dynamic> json) =>
      _$ApiKeyModelFromJson(json);
}

/// ApiKeyCreatedModel — ответ при создании (содержит сырой ключ)
@freezed
abstract class ApiKeyCreatedModel with _$ApiKeyCreatedModel {
  const factory ApiKeyCreatedModel({
    required String id,
    required String name,
    @JsonKey(name: 'key_prefix') required String keyPrefix,
    required String scopes,
    @JsonKey(name: 'expires_at') DateTime? expiresAt,
    @JsonKey(name: 'last_used_at') DateTime? lastUsedAt,
    @JsonKey(name: 'created_at') required DateTime createdAt,
    @JsonKey(name: 'raw_key') required String rawKey,
  }) = _ApiKeyCreatedModel;

  const ApiKeyCreatedModel._();

  factory ApiKeyCreatedModel.fromJson(Map<String, dynamic> json) =>
      _$ApiKeyCreatedModelFromJson(json);
}

/// MCPConfigModel — конфигурация для подключения к MCP-серверу
@freezed
abstract class MCPConfigModel with _$MCPConfigModel {
  const factory MCPConfigModel({
    required Map<String, dynamic> config,
    required String instructions,
    @JsonKey(name: 'server_url') required String serverUrl,
  }) = _MCPConfigModel;

  const MCPConfigModel._();

  factory MCPConfigModel.fromJson(Map<String, dynamic> json) =>
      _$MCPConfigModelFromJson(json);
}
