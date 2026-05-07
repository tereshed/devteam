import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:meta/meta.dart';

/// Результат классификации [WsServiceFailure] для задач (12.3): transient vs блок мутаций.
enum TaskRealtimeFailureKind {
  transient,
  terminalMutationBlock,
}

@immutable
class TaskRealtimeFailureMapping {
  const TaskRealtimeFailureMapping({
    required this.kind,
    this.terminalSession,
  });

  final TaskRealtimeFailureKind kind;

  /// При [TaskRealtimeFailureKind.terminalMutationBlock]: сессия + `realtimeMutationBlocked`.
  final RealtimeSessionFailure? terminalSession;
}

/// Единая точка маппинга WS service failure → поля контроллеров задач (12.3 §80).
TaskRealtimeFailureMapping mapWsServiceFailureForTasks(WsServiceFailure failure) {
  return failure.when(
    transient: (_) => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.transient,
    ),
    authExpired: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.terminalMutationBlock,
      terminalSession: RealtimeSessionFailure.authenticationLost(),
    ),
    policyForbidden: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.terminalMutationBlock,
      terminalSession: RealtimeSessionFailure.forbidden(),
    ),
    policySubprotocolMismatch: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.terminalMutationBlock,
      terminalSession: RealtimeSessionFailure.forbidden(),
    ),
    policyCloseCode: (_) => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.terminalMutationBlock,
      terminalSession: RealtimeSessionFailure.forbidden(),
    ),
    tooManyConnectionsTerminal: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.terminalMutationBlock,
      terminalSession: RealtimeSessionFailure.connectionLimitExceeded(),
    ),
    tooManyConnections: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.transient,
    ),
    protocolBroken: () => const TaskRealtimeFailureMapping(
      kind: TaskRealtimeFailureKind.transient,
    ),
  );
}
