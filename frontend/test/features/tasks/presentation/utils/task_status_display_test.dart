@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/l10n/app_localizations_en.dart';

void main() {
  final l10n = AppLocalizationsEn();

  group('taskStatusLabel', () {
    test('maps normative statuses to taskStatus* keys', () {
      expect(taskStatusLabel(l10n, 'pending'), l10n.taskStatusPending);
      expect(taskStatusLabel(l10n, 'planning'), l10n.taskStatusPlanning);
      expect(taskStatusLabel(l10n, 'in_progress'), l10n.taskStatusInProgress);
      expect(taskStatusLabel(l10n, 'review'), l10n.taskStatusReview);
      expect(taskStatusLabel(l10n, 'testing'), l10n.taskStatusTesting);
      expect(
        taskStatusLabel(l10n, 'changes_requested'),
        l10n.taskStatusChangesRequested,
      );
      expect(taskStatusLabel(l10n, 'completed'), l10n.taskStatusCompleted);
      expect(taskStatusLabel(l10n, 'failed'), l10n.taskStatusFailed);
      expect(taskStatusLabel(l10n, 'cancelled'), l10n.taskStatusCancelled);
      expect(taskStatusLabel(l10n, 'paused'), l10n.taskStatusPaused);
    });

    test('unknown wire → taskStatusUnknownStatus', () {
      expect(taskStatusLabel(l10n, ''), l10n.taskStatusUnknownStatus);
      expect(taskStatusLabel(l10n, 'unknown_future'), l10n.taskStatusUnknownStatus);
    });

    test('taskStatuses iterable matches explicit matrix', () {
      for (final s in taskStatuses) {
        expect(taskStatusLabel(l10n, s), isNot(l10n.taskStatusUnknownStatus));
      }
    });
  });

  group('taskPriorityLabel', () {
    test('maps normative priorities', () {
      expect(taskPriorityLabel(l10n, 'critical'), l10n.taskPriorityCritical);
      expect(taskPriorityLabel(l10n, 'high'), l10n.taskPriorityHigh);
      expect(taskPriorityLabel(l10n, 'medium'), l10n.taskPriorityMedium);
      expect(taskPriorityLabel(l10n, 'low'), l10n.taskPriorityLow);
    });

    test('unknown wire → taskPriorityUnknown', () {
      expect(taskPriorityLabel(l10n, ''), l10n.taskPriorityUnknown);
      expect(taskPriorityLabel(l10n, 'urgent'), l10n.taskPriorityUnknown);
    });

    test('taskPriorities iterable matches explicit matrix', () {
      for (final p in taskPriorities) {
        expect(taskPriorityLabel(l10n, p), isNot(l10n.taskPriorityUnknown));
      }
    });
  });
}
