import 'package:freezed_annotation/freezed_annotation.dart';

part 'assistant_message_model.freezed.dart';
part 'assistant_message_model.g.dart';

/// Допустимые значения `role` (см. `AssistantRole` на бэкенде).
const assistantMessageRoles = <String>[
  'user',
  'assistant',
  'tool',
  'system',
];

const assistantMessageRoleUser = 'user';
const assistantMessageRoleAssistant = 'assistant';
const assistantMessageRoleTool = 'tool';
const assistantMessageRoleSystem = 'system';

/// Одно сообщение assistant-сессии (Sprint 21 §2).
///
/// Для role=user/assistant — заполнен только `content`.
/// Для role=tool — заполнены `toolCallId`, `toolName`, `toolArguments`,
/// и (после confirm/исполнения) `toolResult`.
@freezed
abstract class AssistantMessageModel with _$AssistantMessageModel {
  const factory AssistantMessageModel({
    required String id,

    @JsonKey(name: 'session_id')
    required String sessionId,

    /// Роль — строка из [assistantMessageRoles].
    required String role,

    String? content,

    @JsonKey(name: 'tool_call_id')
    String? toolCallId,

    @JsonKey(name: 'tool_name')
    String? toolName,

    @JsonKey(name: 'tool_arguments')
    Map<String, dynamic>? toolArguments,

    @JsonKey(name: 'tool_result')
    Map<String, dynamic>? toolResult,

    @JsonKey(name: 'client_message_id')
    String? clientMessageId,

    @JsonKey(name: 'created_at')
    required DateTime createdAt,
  }) = _AssistantMessageModel;

  const AssistantMessageModel._();

  factory AssistantMessageModel.fromJson(Map<String, dynamic> json) =>
      _$AssistantMessageModelFromJson(json);

  bool get isUser => role == assistantMessageRoleUser;
  bool get isAssistant => role == assistantMessageRoleAssistant;
  bool get isTool => role == assistantMessageRoleTool;
  bool get isSystem => role == assistantMessageRoleSystem;
}

/// Курсорная пагинация истории: `GET /assistant/sessions/:id/messages`.
///
/// Бэкенд возвращает сообщения в порядке (created_at, id) DESC.
/// Для следующей страницы фронт передаёт `(next_before_created_at, next_before_id)`.
@freezed
abstract class AssistantMessageListResponse
    with _$AssistantMessageListResponse {
  const factory AssistantMessageListResponse({
    @Default(<AssistantMessageModel>[]) List<AssistantMessageModel> messages,
    @Default(0) int limit,
    @JsonKey(name: 'has_more') @Default(false) bool hasMore,
    @JsonKey(name: 'next_before_created_at') DateTime? nextBeforeCreatedAt,
    @JsonKey(name: 'next_before_id') String? nextBeforeId,
  }) = _AssistantMessageListResponse;

  factory AssistantMessageListResponse.fromJson(Map<String, dynamic> json) =>
      _$AssistantMessageListResponseFromJson(json);
}

/// Ответ на POST `/assistant/sessions/:id/messages`. `duplicate=true` —
/// idempotent replay: фронт может НЕ показывать typing-индикатор (петля
/// уже отработала ранее).
@freezed
abstract class SendAssistantMessageResponse
    with _$SendAssistantMessageResponse {
  const factory SendAssistantMessageResponse({
    required AssistantMessageModel message,
    @Default(false) bool duplicate,
  }) = _SendAssistantMessageResponse;

  factory SendAssistantMessageResponse.fromJson(Map<String, dynamic> json) =>
      _$SendAssistantMessageResponseFromJson(json);
}

/// Ответ на POST `/assistant/sessions/:id/confirm`.
@freezed
abstract class ConfirmToolCallResponse with _$ConfirmToolCallResponse {
  const factory ConfirmToolCallResponse({
    @Default(false) bool accepted,
  }) = _ConfirmToolCallResponse;

  factory ConfirmToolCallResponse.fromJson(Map<String, dynamic> json) =>
      _$ConfirmToolCallResponseFromJson(json);
}
