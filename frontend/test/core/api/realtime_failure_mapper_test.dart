// @dart=2.19
@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/realtime_failure_mapper.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';

void main() {
  group('mapWsServiceFailureForTasks', () {
    test('transient keeps terminal semantics separate', () {
      final m = mapWsServiceFailureForTasks(const WsServiceFailure.transient());
      expect(m.kind, TaskRealtimeFailureKind.transient);
      expect(m.terminalSession, isNull);
    });

    test('authExpired → terminalMutationBlock', () {
      final m = mapWsServiceFailureForTasks(const WsServiceFailure.authExpired());
      expect(m.kind, TaskRealtimeFailureKind.terminalMutationBlock);
      expect(
        m.terminalSession,
        const RealtimeSessionFailure.authenticationLost(),
      );
    });
  });
}
