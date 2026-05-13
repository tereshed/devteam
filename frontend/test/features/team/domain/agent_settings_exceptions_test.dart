import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/team/domain/agent_settings_exceptions.dart';

void main() {
  test('subclasses are AgentSettingsException + Exception', () {
    final List<AgentSettingsException> list = [
      AgentSettingsCancelledException('c'),
      AgentSettingsForbiddenException('f'),
      AgentSettingsNotFoundException('n'),
      AgentSettingsConflictException('c2'),
      AgentSettingsApiException('a', statusCode: 500),
    ];
    for (final e in list) {
      expect(e, isA<AgentSettingsException>());
      expect(e, isA<Exception>());
    }
  });

  test('Forbidden equality respects apiErrorCode', () {
    final a = AgentSettingsForbiddenException('x', apiErrorCode: 'denied');
    final b = AgentSettingsForbiddenException('x', apiErrorCode: 'denied');
    expect(a, equals(b));
    expect(a.hashCode, equals(b.hashCode));
  });

  test('different message → not equal', () {
    final a = AgentSettingsNotFoundException('agent A');
    final b = AgentSettingsNotFoundException('agent B');
    expect(a, isNot(equals(b)));
  });
}
