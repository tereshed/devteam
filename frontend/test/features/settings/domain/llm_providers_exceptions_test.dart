import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/settings/domain/llm_providers_exceptions.dart';

/// Sprint 15.M7 — иерархия + ==/hashCode для исключений LLM-провайдеров.
void main() {
  test('LLMProvidersForbiddenException equality and hashCode', () {
    final a = LLMProvidersForbiddenException('nope', apiErrorCode: 'admin_only');
    final b = LLMProvidersForbiddenException('nope', apiErrorCode: 'admin_only');
    expect(a, equals(b));
    expect(a.hashCode, equals(b.hashCode));
  });

  test('different apiErrorCode → not equal', () {
    final a = LLMProvidersForbiddenException('nope', apiErrorCode: 'x');
    final b = LLMProvidersForbiddenException('nope', apiErrorCode: 'y');
    expect(a, isNot(equals(b)));
  });

  test('originalError not considered in equality', () {
    final a = LLMProvidersConflictException('x',
        apiErrorCode: 'dup', originalError: Exception('orig-1'));
    final b = LLMProvidersConflictException('x',
        apiErrorCode: 'dup', originalError: Exception('orig-2'));
    expect(a, equals(b));
  });

  test('hierarchy: all subclasses are LLMProvidersException', () {
    final List<LLMProvidersException> list = [
      LLMProvidersCancelledException('c'),
      LLMProvidersForbiddenException('f'),
      LLMProvidersNotFoundException('n'),
      LLMProvidersConflictException('co'),
      LLMProvidersApiException('a', statusCode: 500),
    ];
    for (final e in list) {
      expect(e, isA<LLMProvidersException>());
      expect(e, isA<Exception>());
    }
  });

  test('toString returns message', () {
    final e = LLMProvidersForbiddenException('blocked');
    expect(e.toString(), 'Forbidden: blocked');
  });
}
