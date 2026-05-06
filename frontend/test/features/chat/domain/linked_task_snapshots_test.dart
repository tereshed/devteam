// @dart=2.19
@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/chat/domain/linked_task_snapshots.dart';

void main() {
  group('LinkedTaskSnapshotPatch merge', () {
    test('absent leaves previous error_message', () {
      const base = LinkedTaskSnapshot(
        taskId: 't1',
        status: 'pending',
        errorMessage: 'old',
      );
      const patch = LinkedTaskSnapshotPatch(
        status: SnapshotPresent('in_progress'),
      );
      final out = mergeTaskSnapshotPatch(base, patch);
      expect(out.errorMessage, 'old');
      expect(out.status, 'in_progress');
    });

    test('explicit null clears error_message', () {
      const base = LinkedTaskSnapshot(
        taskId: 't1',
        status: 'failed',
        errorMessage: 'boom',
      );
      const patch = LinkedTaskSnapshotPatch(
        errorMessage: SnapshotPresent<String?>(null),
      );
      final out = mergeTaskSnapshotPatch(base, patch);
      expect(out.errorMessage, isNull);
    });

    test('present string overwrites title', () {
      const base = LinkedTaskSnapshot(
        taskId: 't1',
        title: 'a',
        status: 'pending',
      );
      const patch = LinkedTaskSnapshotPatch(
        title: SnapshotPresent<String?>('b'),
      );
      final out = mergeTaskSnapshotPatch(base, patch);
      expect(out.title, 'b');
    });

    test(
      'fromWsTaskStatus: wire omits absent/null distinction — null clears prior error',
      () {
        const base = LinkedTaskSnapshot(
          taskId: 't1',
          status: 'failed',
          errorMessage: 'boom',
        );
        final ev = WsTaskStatusEvent(
          ts: DateTime.utc(2026, 2, 1),
          v: 1,
          projectId: 'p',
          taskId: 't1',
          previousStatus: 'failed',
          status: 'in_progress',
          errorMessage: null,
        );
        final out = mergeTaskSnapshotPatch(
          base,
          LinkedTaskSnapshotPatch.fromWsTaskStatus(ev),
        );
        expect(out.errorMessage, isNull);
        expect(out.status, 'in_progress');
      },
    );
  });
}
