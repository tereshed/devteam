import 'package:freezed_annotation/freezed_annotation.dart';

part 'tool_definition_model.freezed.dart';
part 'tool_definition_model.g.dart';

/// Элемент каталога GET /tool-definitions (read-only).
@freezed
abstract class ToolDefinitionModel with _$ToolDefinitionModel {
  const factory ToolDefinitionModel({
    required String id,
    required String name,
    required String description,
    required String category,
    @JsonKey(name: 'is_builtin') required bool isBuiltin,
  }) = _ToolDefinitionModel;

  factory ToolDefinitionModel.fromJson(Map<String, dynamic> json) =>
      _$ToolDefinitionModelFromJson(json);
}
