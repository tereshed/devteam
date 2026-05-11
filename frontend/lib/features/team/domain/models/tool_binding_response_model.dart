import 'package:freezed_annotation/freezed_annotation.dart';

part 'tool_binding_response_model.freezed.dart';
part 'tool_binding_response_model.g.dart';

/// Привязка инструмента к агенту в ответе GET …/team (read-only).
@freezed
abstract class ToolBindingResponseModel with _$ToolBindingResponseModel {
  const factory ToolBindingResponseModel({
    @JsonKey(name: 'tool_definition_id') required String toolDefinitionId,
    required String name,
    required String category,
  }) = _ToolBindingResponseModel;

  factory ToolBindingResponseModel.fromJson(Map<String, dynamic> json) =>
      _$ToolBindingResponseModelFromJson(json);
}
