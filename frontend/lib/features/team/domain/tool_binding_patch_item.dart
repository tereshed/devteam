import 'package:freezed_annotation/freezed_annotation.dart';

part 'tool_binding_patch_item.freezed.dart';
part 'tool_binding_patch_item.g.dart';

/// Элемент массива `tool_bindings` в PATCH агента (write-only).
@freezed
abstract class ToolBindingPatchItem with _$ToolBindingPatchItem {
  const factory ToolBindingPatchItem({
    @JsonKey(name: 'tool_definition_id') required String toolDefinitionId,
  }) = _ToolBindingPatchItem;

  factory ToolBindingPatchItem.fromJson(Map<String, dynamic> json) =>
      _$ToolBindingPatchItemFromJson(json);
}
