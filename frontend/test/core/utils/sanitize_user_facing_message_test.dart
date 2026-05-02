import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/utils/sanitize_user_facing_message.dart';

void main() {
  group('sanitizeUserFacingMessage', () {
    test('removes userinfo from https URL in text', () {
      const input = 'failed https://u:tok@host.example/path end';
      final out = sanitizeUserFacingMessage(input);
      expect(out, isNot(contains('tok@')));
      expect(out, isNot(contains('u:tok')));
      expect(out, contains('host.example'));
    });

    test('leaves URL without userinfo unchanged', () {
      const s = 'see https://github.com/foo/bar';
      expect(sanitizeUserFacingMessage(s), s);
    });
  });
}
