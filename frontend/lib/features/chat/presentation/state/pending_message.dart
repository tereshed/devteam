import 'package:freezed_annotation/freezed_annotation.dart';

part 'pending_message.freezed.dart';

@freezed
abstract class PendingMessage with _$PendingMessage {
  const factory PendingMessage({
    required String clientMessageId,
    required String content,
    Object? lastError,
    @Default(1) int attempts,
    required DateTime lastAttemptAt,
  }) = _PendingMessage;

  const PendingMessage._();
}
