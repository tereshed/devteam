@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/presentation/utils/task_agent_role_display.dart';
import 'package:frontend/l10n/app_localizations_en.dart';

void main() {
  final l10n = AppLocalizationsEn();

  test('каждая agentRoles → осмысленный taskAgentRole* (не unknown)', () {
    for (final r in agentRoles) {
      final label = taskAgentRoleLabel(l10n, r);
      expect(label, isNot(l10n.taskAgentRoleUnknown));
      expect(label, isNotEmpty);
    }
    expect(taskAgentRoleLabel(l10n, 'future_role'), l10n.taskAgentRoleUnknown);
  });
}
