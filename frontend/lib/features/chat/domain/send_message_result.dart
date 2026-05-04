import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:frontend/features/chat/domain/models/conversation_message_model.dart';

part 'send_message_result.freezed.dart';

/// Статус ответа [ConversationRepository.sendMessage] (201 — создано, 200 — идемпотентный дубликат).
enum MessageSendStatus {
  created,
  duplicate,
}

@freezed
abstract class SendMessageResult with _$SendMessageResult {
  const factory SendMessageResult({
    required ConversationMessageModel message,
    required MessageSendStatus status,
  }) = _SendMessageResult;
}
