import 'package:freezed_annotation/freezed_annotation.dart';

part 'task_message_model.freezed.dart';
part 'task_message_model.g.dart';

/// Допустимые значения `sender_type` (`SenderType` на бэкенде).
const senderTypes = ['user', 'agent'];

/// Значение [senderTypes] для пользователя.
const kSenderTypeUser = 'user';

/// Значение [senderTypes] для агента.
const kSenderTypeAgent = 'agent';

/// Допустимые значения `message_type` (`MessageType` на бэкенде).
const messageTypes = [
  'instruction',
  'result',
  'question',
  'feedback',
  'error',
  'comment',
  'summary',
];

/// Значение [messageTypes] для инструкции.
const kMessageTypeInstruction = 'instruction';

/// Значение [messageTypes] для результата.
const kMessageTypeResult = 'result';

/// Значение [messageTypes] для ошибки в логе задачи.
const kMessageTypeError = 'error';

/// Значение [messageTypes] для обратной связи.
const kMessageTypeFeedback = 'feedback';

/// Сообщение в контексте задачи (`TaskMessageResponse` в API).
@freezed
abstract class TaskMessageModel with _$TaskMessageModel {
  const factory TaskMessageModel({
    /// UUID сообщения
    required String id,

    /// UUID задачи
    @JsonKey(name: 'task_id')
    required String taskId,

    /// `user` | `agent` — см. [senderTypes]
    @JsonKey(name: 'sender_type')
    required String senderType,

    /// UUID отправителя; семантика — [senderType]
    @JsonKey(name: 'sender_id')
    required String senderId,

    /// Текст сообщения
    required String content,

    /// Тип сообщения — см. [messageTypes]
    @JsonKey(name: 'message_type')
    required String messageType,

    /// Метаданные (JSONB)
    @Default(<String, dynamic>{})
    Map<String, dynamic> metadata,

    /// Дата создания
    @JsonKey(name: 'created_at')
    required DateTime createdAt,
  }) = _TaskMessageModel;

  const TaskMessageModel._();

  factory TaskMessageModel.fromJson(Map<String, dynamic> json) =>
      _$TaskMessageModelFromJson(json);
}
