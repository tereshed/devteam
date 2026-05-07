import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/tasks/domain/models.dart';

/// WS → доменная модель (вариант **а** по Sprint 12.3 §49): [WsTaskMessageEvent.senderRole]
/// сознательно не переносится — нет поля в [TaskMessageModel]; в [metadata] не кладём.
TaskMessageModel wsTaskMessageToModel(
  WsTaskMessageEvent e, {
  required String taskId,
}) {
  return TaskMessageModel(
    id: e.messageId,
    taskId: taskId,
    senderType: e.senderType,
    senderId: e.senderId,
    content: e.content,
    messageType: e.messageType,
    metadata: Map<String, dynamic>.from(e.metadata),
    createdAt: e.ts,
  );
}
