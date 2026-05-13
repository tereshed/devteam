import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/settings/domain/claude_code_auth_exceptions.dart';

/// Sprint 15.M7 — exception иерархия Claude Code OAuth.
void main() {
  test('all subclasses extend ClaudeCodeAuthException and Exception', () {
    final List<ClaudeCodeAuthException> list = [
      ClaudeCodeAuthCancelledException('c'),
      ClaudeCodeAuthorizationPendingException('p'),
      ClaudeCodeAuthSlowDownException('s'),
      ClaudeCodeAuthFlowEndedException('e', apiErrorCode: 'expired_token'),
      ClaudeCodeAuthOwnerMismatchException('o'),
      ClaudeCodeAuthApiException('a', statusCode: 500),
    ];
    for (final e in list) {
      expect(e, isA<ClaudeCodeAuthException>());
      expect(e, isA<Exception>());
    }
  });

  test('AuthorizationPending equality without apiErrorCode', () {
    final a = ClaudeCodeAuthorizationPendingException('pending');
    final b = ClaudeCodeAuthorizationPendingException('pending');
    expect(a, equals(b));
    expect(a.hashCode, equals(b.hashCode));
  });

  test('FlowEnded differentiates expired vs access_denied via apiErrorCode', () {
    final expired = ClaudeCodeAuthFlowEndedException('end',
        apiErrorCode: 'expired_token');
    final denied = ClaudeCodeAuthFlowEndedException('end',
        apiErrorCode: 'access_denied');
    expect(expired, isNot(equals(denied)));
  });

  test('OwnerMismatch — single matcher для router/UI', () {
    final a = ClaudeCodeAuthOwnerMismatchException('mismatch');
    expect(a, isA<ClaudeCodeAuthOwnerMismatchException>());
    expect(a, isA<ClaudeCodeAuthException>());
  });
}
