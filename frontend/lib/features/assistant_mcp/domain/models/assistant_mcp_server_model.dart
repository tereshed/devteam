import 'package:freezed_annotation/freezed_annotation.dart';

part 'assistant_mcp_server_model.freezed.dart';
part 'assistant_mcp_server_model.g.dart';

/// Внешний MCP-сервер ассистента проекта. Зеркалит backend
/// `dto.AssistantMCPServerResponse`. Remote-only: transport `http` | `sse`.
/// В [headers] значения могут содержать `${secret:NAME}` (резолвятся бэкендом из
/// «Переменных проекта» — реальные секреты тут не хранятся).
@freezed
abstract class AssistantMcpServerModel with _$AssistantMcpServerModel {
  const factory AssistantMcpServerModel({
    @Default('') String id,
    @JsonKey(name: 'project_id') @Default('') String projectId,
    @Default('') String name,
    @Default('http') String transport,
    @Default('') String url,
    @Default(<String, String>{}) Map<String, String> headers,
    @JsonKey(name: 'require_confirmation')
    @Default(true)
    bool requireConfirmation,
    @JsonKey(name: 'is_enabled') @Default(true) bool isEnabled,
  }) = _AssistantMcpServerModel;

  factory AssistantMcpServerModel.fromJson(Map<String, dynamic> json) =>
      _$AssistantMcpServerModelFromJson(json);
}
