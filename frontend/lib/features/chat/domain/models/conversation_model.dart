import 'package:freezed_annotation/freezed_annotation.dart';

part 'conversation_model.freezed.dart';
part 'conversation_model.g.dart';

/// Допустимые значения `status` (см. `models.ConversationStatus` на бэкенде).
const conversationStatuses = [
  'active',
  'completed',
  'archived',
];

@freezed
abstract class ConversationModel with _$ConversationModel {
  const factory ConversationModel({
    /// UUID чата
    required String id,

    /// UUID проекта
    @JsonKey(name: 'project_id')
    required String projectId,

    /// Заголовок чата (автогенерация / пользовательский)
    required String title,

    /// Статус — строка из [conversationStatuses] (backend `models.ConversationStatus`).
    required String status,

    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _ConversationModel;

  const ConversationModel._();

  factory ConversationModel.fromJson(Map<String, dynamic> json) =>
      _$ConversationModelFromJson(json);
}
