import 'package:freezed_annotation/freezed_annotation.dart';

part 'conversation_message_model.freezed.dart';
part 'conversation_message_model.g.dart';

/// Нормализация из JSON: канонический флаг стрима — `streaming`; legacy API может слать `is_streaming`.
Map<String, dynamic>? _conversationMetadataFromJson(Object? json) {
  if (json == null) {
    return null;
  }
  if (json is! Map) {
    return null;
  }
  final m = Map<String, dynamic>.from(
    json.map((key, value) => MapEntry(key.toString(), value)),
  );
  if (!m.containsKey('streaming') && m.containsKey('is_streaming')) {
    m['streaming'] = m['is_streaming'];
  }
  return m;
}

/// Допустимые значения `role` (см. `ConversationRole` на бэкенде).
const conversationMessageRoles = [
  'user',
  'assistant',
  'system',
];

@freezed
abstract class ConversationMessageModel with _$ConversationMessageModel {
  const factory ConversationMessageModel({
    required String id,

    @JsonKey(name: 'conversation_id')
    required String conversationId,

    /// Роль — строка из [conversationMessageRoles] (backend `ConversationRole`).
    required String role,

    required String content,

    /// Как `tech_stack` / `settings` в `project_model.dart`: @Default задаёт значение и для fromJson (нет ключа), и для ручного конструктора.
    @JsonKey(name: 'linked_task_ids')
    @Default(<String>[])
    List<String> linkedTaskIds,

    @JsonKey(name: 'metadata', fromJson: _conversationMetadataFromJson)
    Map<String, dynamic>? metadata,

    @JsonKey(name: 'created_at')
    required DateTime createdAt,
  }) = _ConversationMessageModel;

  const ConversationMessageModel._();

  factory ConversationMessageModel.fromJson(Map<String, dynamic> json) =>
      _$ConversationMessageModelFromJson(json);
}
