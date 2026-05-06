import 'package:flutter/foundation.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/chat/domain/models/conversation_message_model.dart';

/// Ключ JSON в [ConversationMessageModel.metadata] для снимков связанных задач.
const String kLinkedTaskSnapshotsMetadataKey = 'linked_task_snapshots';

/// Поле присутствует в патче: либо значение (в т.ч. явный `null` — очистить), либо отсутствует.
@immutable
sealed class SnapshotPatchField<T> {
  const SnapshotPatchField._();

  bool get isAbsent => this is SnapshotAbsent<T>;
}

/// Ключ поля нет в патче — старое значение сохраняется.
final class SnapshotAbsent<T> extends SnapshotPatchField<T> {
  const SnapshotAbsent() : super._();
}

/// Явное значение для поля, включая `null` для очистки в снимке.
final class SnapshotPresent<T> extends SnapshotPatchField<T> {
  const SnapshotPresent(this.value) : super._();
  final T? value;
}

/// Частичное обновление снимка задачи из WS `task_status` или REST.
@immutable
class LinkedTaskSnapshotPatch {
  const LinkedTaskSnapshotPatch({
    this.status = const SnapshotAbsent<String>(),
    this.title = const SnapshotAbsent<String?>(),
    this.errorMessage = const SnapshotAbsent<String?>(),
    this.agentRoleRaw = const SnapshotAbsent<String?>(),
  });

  final SnapshotPatchField<String> status;
  final SnapshotPatchField<String?> title;
  final SnapshotPatchField<String?> errorMessage;
  final SnapshotPatchField<String?> agentRoleRaw;

  /// Поля из [WsTaskStatusEvent] ([title] по политике 11.9 не обновляется через WS).
  ///
  /// Wire 7.3 не различает «ключ отсутствует» и `null` для опциональных полей —
  /// здесь [errorMessage]/[agentRole] трактуются как **всегда присутствующие**
  /// ([SnapshotPresent]): значение `null` после успешного парсинга означает
  /// очистку предыдущего снимка (в т.ч. переход failed→in_progress без текста ошибки).
  factory LinkedTaskSnapshotPatch.fromWsTaskStatus(WsTaskStatusEvent e) {
    return LinkedTaskSnapshotPatch(
      status: SnapshotPresent(e.status),
      title: const SnapshotAbsent<String?>(),
      errorMessage: SnapshotPresent(e.errorMessage),
      agentRoleRaw: SnapshotPresent(e.agentRole),
    );
  }
}

/// Снимок полей задачи для карточки из `metadata.linked_task_snapshots[task_id]`.
@immutable
class LinkedTaskSnapshot {
  const LinkedTaskSnapshot({
    required this.taskId,
    this.title,
    this.status = '',
    this.errorMessage,
    this.agentRoleRaw,
  });

  final String taskId;
  final String? title;
  final String status;
  final String? errorMessage;
  final String? agentRoleRaw;

  Map<String, dynamic> toMetadataEntryJson() {
    return <String, dynamic>{
      if (title != null) 'title': title,
      'status': status,
      if (errorMessage != null) 'error_message': errorMessage,
      if (agentRoleRaw != null) 'agent_role': agentRoleRaw,
    };
  }
}

/// Читает карту `task_id → объект` из [metadata].
Map<String, LinkedTaskSnapshot>? readLinkedTaskSnapshotsFromMetadata(
  Map<String, dynamic>? metadata,
) {
  if (metadata == null) {
    return null;
  }
  final v = metadata[kLinkedTaskSnapshotsMetadataKey];
  if (v is! Map) {
    return null;
  }
  final out = <String, LinkedTaskSnapshot>{};
  for (final e in v.entries) {
    final taskId = e.key.toString();
    final raw = e.value;
    if (raw is! Map) {
      continue;
    }
    final map = Map<String, dynamic>.from(
      raw.map((k, v) => MapEntry(k.toString(), v)),
    );
    assert(() {
      for (final jsonKey in <String>['status', 'title', 'error_message', 'agent_role']) {
        final x = map[jsonKey];
        if (x != null && x is! String) {
          throw FlutterError(
            'linked_task_snapshots[$taskId].$jsonKey: expected String?, got ${x.runtimeType}',
          );
        }
      }
      return true;
    }());
    out[taskId] = LinkedTaskSnapshot(
      taskId: taskId,
      title: map['title'] as String?,
      status: map['status'] as String? ?? '',
      errorMessage: map['error_message'] as String?,
      agentRoleRaw: map['agent_role'] as String?,
    );
  }
  return out;
}

/// Снимок для одной связанной задачи (объединяет REST metadata и дефолты).
LinkedTaskSnapshot linkedTaskSnapshotForMessage(
  ConversationMessageModel message,
  String taskId,
) {
  final snaps = readLinkedTaskSnapshotsFromMetadata(message.metadata);
  if (snaps != null) {
    final s = snaps[taskId];
    if (s != null) {
      return s;
    }
  }
  return LinkedTaskSnapshot(taskId: taskId, title: null, status: '');
}

String? _mergeNullableString(String? current, SnapshotPatchField<String?> patch) {
  return switch (patch) {
    SnapshotAbsent<String?>() => current,
    SnapshotPresent<String?>(:final value) => value,
  };
}

/// Семантика partial-patch: absent → оставить; present(null) → очистить.
LinkedTaskSnapshot mergeTaskSnapshotPatch(
  LinkedTaskSnapshot current,
  LinkedTaskSnapshotPatch patch,
) {
  return LinkedTaskSnapshot(
    taskId: current.taskId,
    title: _mergeNullableString(current.title, patch.title),
    status: _mergeStatus(current.status, patch.status),
    errorMessage: _mergeNullableString(current.errorMessage, patch.errorMessage),
    agentRoleRaw: _mergeNullableString(current.agentRoleRaw, patch.agentRoleRaw),
  );
}

String _mergeStatus(String current, SnapshotPatchField<String> patch) {
  return switch (patch) {
    SnapshotAbsent<String>() => current,
    SnapshotPresent<String>(:final value) => value ?? '',
  };
}

/// Применяет патч к [message] для [taskId] в `metadata.linked_task_snapshots`.
ConversationMessageModel applyLinkedSnapshotPatchToMessage(
  ConversationMessageModel message,
  String taskId,
  LinkedTaskSnapshotPatch patch,
) {
  final existing = readLinkedTaskSnapshotsFromMetadata(message.metadata) ?? {};
  final prev =
      existing[taskId] ?? LinkedTaskSnapshot(taskId: taskId, title: null, status: '');
  final merged = mergeTaskSnapshotPatch(prev, patch);
  final next = Map<String, LinkedTaskSnapshot>.from(existing)..[taskId] = merged;
  final meta = Map<String, dynamic>.from(message.metadata ?? <String, dynamic>{});
  meta[kLinkedTaskSnapshotsMetadataKey] = <String, dynamic>{
    for (final e in next.entries) e.key: e.value.toMetadataEntryJson(),
  };
  return message.copyWith(metadata: meta);
}
