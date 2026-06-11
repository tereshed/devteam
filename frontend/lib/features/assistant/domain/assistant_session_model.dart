import 'package:freezed_annotation/freezed_annotation.dart';

part 'assistant_session_model.freezed.dart';
part 'assistant_session_model.g.dart';

/// Допустимые значения `status` (см. `models.AssistantSessionStatus` на бэкенде).
const assistantSessionStatuses = <String>[
  'active',
  'archived',
];

const assistantSessionStatusActive = 'active';
const assistantSessionStatusArchived = 'archived';

/// Сессия глобального ассистента (Sprint 21 §2).
///
/// `projectId` — scope сессии: null = глобальная (sidebar вне проекта), иначе
/// сессия привязана к проекту и получает PROJECT CONTEXT в промпт. Поля `busy`,
/// `busySince`, `pendingToolCallId`
/// нужны UI для дизейбла input до прихода `assistant.session_updated busy=false`
/// (план §3.1, §4.1).
@freezed
abstract class AssistantSessionModel with _$AssistantSessionModel {
  const factory AssistantSessionModel({
    required String id,

    @JsonKey(name: 'user_id')
    required String userId,

    /// Scope: null — глобальная сессия, иначе id проекта.
    @JsonKey(name: 'project_id')
    String? projectId,

    String? title,

    /// Статус — строка из [assistantSessionStatuses].
    required String status,

    required bool busy,

    @JsonKey(name: 'busy_since')
    DateTime? busySince,

    @JsonKey(name: 'pending_tool_call_id')
    String? pendingToolCallId,

    @JsonKey(name: 'metadata')
    Map<String, dynamic>? metadata,

    @JsonKey(name: 'last_message_at')
    DateTime? lastMessageAt,

    @JsonKey(name: 'created_at')
    required DateTime createdAt,

    @JsonKey(name: 'updated_at')
    required DateTime updatedAt,
  }) = _AssistantSessionModel;

  const AssistantSessionModel._();

  factory AssistantSessionModel.fromJson(Map<String, dynamic> json) =>
      _$AssistantSessionModelFromJson(json);
}

/// Обёртка списка сессий: `GET /assistant/sessions`.
@freezed
abstract class AssistantSessionListResponse
    with _$AssistantSessionListResponse {
  const factory AssistantSessionListResponse({
    @Default(<AssistantSessionModel>[]) List<AssistantSessionModel> sessions,
  }) = _AssistantSessionListResponse;

  factory AssistantSessionListResponse.fromJson(Map<String, dynamic> json) =>
      _$AssistantSessionListResponseFromJson(json);
}
