import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/presentation/utils/git_remote_url.dart';

void main() {
  group('isValidGitRemoteUrl', () {
    test('accepts https URL', () {
      expect(
        isValidGitRemoteUrl('https://github.com/o/r.git'),
        isTrue,
      );
    });

    test('accepts scp-style git@host:path', () {
      expect(
        isValidGitRemoteUrl('git@github.com:org/repo.git'),
        isTrue,
      );
    });

    test('accepts ssh:// URL', () {
      expect(
        isValidGitRemoteUrl('ssh://git@github.com/user/repo.git'),
        isTrue,
      );
    });

    test('rejects unsupported scheme, plain text, empty host', () {
      expect(isValidGitRemoteUrl('ftp://host/repo'), isFalse);
      expect(isValidGitRemoteUrl('just-a-string'), isFalse);
      expect(isValidGitRemoteUrl('http://'), isFalse);
    });

    test('rejects empty and interior spaces', () {
      expect(isValidGitRemoteUrl(''), isFalse);
      expect(isValidGitRemoteUrl('  '), isFalse);
      expect(isValidGitRemoteUrl('https://a b.com/x'), isFalse);
    });
  });
}
