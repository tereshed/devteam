import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/chat/domain/models.dart';

part 'requests.freezed.dart';
part 'requests.g.dart';

@freezed
abstract class ConversationListResponse with _$ConversationListResponse {
  const factory ConversationListResponse({
    @Default(<ConversationModel>[]) List<ConversationModel> conversations,
    @Default(0) int total,
    @Default(0) int limit,
    @Default(0) int offset,
    @JsonKey(name: 'has_next')
    @Default(false)
    bool hasNext,
  }) = _ConversationListResponse;

  const ConversationListResponse._();

  factory ConversationListResponse.fromJson(Map<String, dynamic> json) =>
      _$ConversationListResponseFromJson(json);
}

@freezed
abstract class MessageListResponse with _$MessageListResponse {
  const factory MessageListResponse({
    @Default(<ConversationMessageModel>[])
    List<ConversationMessageModel> messages,
    @Default(0) int total,
    @Default(0) int limit,
    @Default(0) int offset,
    @JsonKey(name: 'has_next')
    @Default(false)
    bool hasNext,
  }) = _MessageListResponse;

  const MessageListResponse._();

  factory MessageListResponse.fromJson(Map<String, dynamic> json) =>
      _$MessageListResponseFromJson(json);
}
