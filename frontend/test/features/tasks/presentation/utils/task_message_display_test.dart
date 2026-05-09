@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_message_model.dart';
import 'package:frontend/features/tasks/presentation/utils/task_message_display.dart';
import 'package:frontend/l10n/app_localizations_en.dart';

void main() {
  final l10n = AppLocalizationsEn();

  group('taskMessageTypeLabel', () {
    test('matrix messageTypes', () {
      expect(taskMessageTypeLabel(l10n, 'instruction'), l10n.taskMessageTypeInstruction);
      expect(taskMessageTypeLabel(l10n, 'result'), l10n.taskMessageTypeResult);
      expect(taskMessageTypeLabel(l10n, 'question'), l10n.taskMessageTypeQuestion);
      expect(taskMessageTypeLabel(l10n, 'feedback'), l10n.taskMessageTypeFeedback);
      expect(taskMessageTypeLabel(l10n, 'error'), l10n.taskMessageTypeError);
      expect(taskMessageTypeLabel(l10n, 'comment'), l10n.taskMessageTypeComment);
      expect(taskMessageTypeLabel(l10n, 'summary'), l10n.taskMessageTypeSummary);
    });

    test('unknown → taskMessageTypeUnknown', () {
      expect(taskMessageTypeLabel(l10n, ''), l10n.taskMessageTypeUnknown);
      expect(taskMessageTypeLabel(l10n, 'new_type'), l10n.taskMessageTypeUnknown);
    });

    test('messageTypes нормативный список не даёт unknown', () {
      for (final t in messageTypes) {
        expect(taskMessageTypeLabel(l10n, t), isNot(l10n.taskMessageTypeUnknown));
      }
    });
  });

  group('taskSenderTypeLabel', () {
    test('matrix senderTypes', () {
      expect(taskSenderTypeLabel(l10n, 'user'), l10n.taskSenderTypeUser);
      expect(taskSenderTypeLabel(l10n, 'agent'), l10n.taskSenderTypeAgent);
    });

    test('unknown → taskSenderTypeUnknown', () {
      expect(taskSenderTypeLabel(l10n, 'bot'), l10n.taskSenderTypeUnknown);
    });
  });
}
