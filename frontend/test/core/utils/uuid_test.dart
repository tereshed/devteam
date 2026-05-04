import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/utils/uuid.dart';

void main() {
  group('isValidUuid', () {
    test('принимает типичный UUID v4', () {
      expect(
        isValidUuid('550e8400-e29b-41d4-a716-446655440000'),
        isTrue,
      );
    });

    test('принимает UUID с версией/вариантом вне RFC v1–v5 (например v7)', () {
      expect(
        isValidUuid('018e0bdb-a35b-7a3a-9c0d-8f4e2b1a0c9d'),
        isTrue,
      );
    });

    test('отклоняет не-hex и неверную длину', () {
      expect(isValidUuid('550e8400-e29b-41d4-a716-44665544000'), isFalse);
      expect(isValidUuid('zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz'), isFalse);
      expect(isValidUuid(''), isFalse);
    });
  });
}
