@TestOn('vm')
@Tags(['unit'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/l10n/app_localizations_en.dart';

void main() {
  final l10n = AppLocalizationsEn();

  test('каждая agentRoles → осмысленный agentRole* (не Unknown)', () {
    for (final r in agentRoles) {
      final label = agentRoleLabel(l10n, r);
      expect(label, isNot(l10n.agentRoleUnknown));
      expect(label, isNotEmpty);
    }
    expect(agentRoleLabel(l10n, 'future_role'), l10n.agentRoleUnknown);
  });
}
