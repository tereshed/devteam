import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';

void main() {
  group('ConversationNotFoundException', () {
    test('value equality по message и apiErrorCode', () {
      final a = ConversationNotFoundException(
        'x',
        apiErrorCode: 'not_found',
      );
      final b = ConversationNotFoundException(
        'x',
        apiErrorCode: 'not_found',
      );
      final c = ConversationNotFoundException(
        'x',
        apiErrorCode: 'gone',
      );
      expect(a, equals(b));
      expect(a, isNot(equals(c)));
      expect(a.hashCode, b.hashCode);
    });
  });

  group('ConversationApiException', () {
    test('value equality по message, statusCode и apiErrorCode', () {
      final a = ConversationApiException(
        'x',
        statusCode: 422,
        apiErrorCode: 'validation_error',
      );
      final b = ConversationApiException(
        'x',
        statusCode: 422,
        apiErrorCode: 'validation_error',
      );
      final c = ConversationApiException(
        'x',
        statusCode: 422,
        apiErrorCode: 'other',
      );
      expect(a, equals(b));
      expect(a, isNot(equals(c)));
      expect(a.hashCode, b.hashCode);
    });

    test('originalError не участвует в равенстве', () {
      final a = ConversationApiException(
        'm',
        statusCode: 500,
        apiErrorCode: 'e',
        originalError: StateError('1'),
      );
      final b = ConversationApiException(
        'm',
        statusCode: 500,
        apiErrorCode: 'e',
        originalError: StateError('2'),
      );
      expect(a, equals(b));
    });
  });
}
