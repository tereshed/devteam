// @dart=2.19
@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tasks_panel.dart';
import 'package:frontend/l10n/app_localizations_en.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';

void main() {
  group('assistantTaskStateLabel (V2 only)', () {
    final en = AppLocalizationsEn();
    final ru = AppLocalizationsRu();

    test('active → localized', () {
      expect(assistantTaskStateLabel(en, kAssistantTaskStateActive), 'Active');
      expect(assistantTaskStateLabel(ru, kAssistantTaskStateActive),
          'В работе');
    });

    test('done → localized', () {
      expect(assistantTaskStateLabel(en, kAssistantTaskStateDone), 'Done');
      expect(assistantTaskStateLabel(ru, kAssistantTaskStateDone), 'Готово');
    });

    test('failed / cancelled → localized', () {
      expect(
          assistantTaskStateLabel(en, kAssistantTaskStateFailed), 'Failed');
      expect(assistantTaskStateLabel(en, kAssistantTaskStateCancelled),
          'Cancelled');
    });

    test('needs_human → localized', () {
      expect(assistantTaskStateLabel(en, kAssistantTaskStateNeedsHuman),
          'Needs human');
    });

    test('paused → localized', () {
      expect(
          assistantTaskStateLabel(en, kAssistantTaskStatePaused), 'Paused');
    });

    test('unknown / legacy V1 state → raw fallback (no crash)', () {
      // Legacy V1 значения после Sprint 5 в БД невозможны. Если такое всё
      // же придёт — отдаём как есть, чтобы было видно отладчику. Тест
      // фиксирует контракт «не показываем русско-кириллический хардкод».
      expect(assistantTaskStateLabel(en, 'in_progress'), 'in_progress');
      expect(assistantTaskStateLabel(en, 'completed'), 'completed');
      expect(assistantTaskStateLabel(en, ''), '');
    });
  });
}
