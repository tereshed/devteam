// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/domain/models.dart';

void main() {
  group('ConversationModel', () {
    test('conversationStatuses совпадает с backend ConversationStatus', () {
      expect(
        conversationStatuses,
        equals(<String>['active', 'completed', 'archived']),
      );
    });

    final validJson = <String, dynamic>{
      'id': '550e8400-e29b-41d4-a716-446655440000',
      'project_id': '6ba7b810-9dad-11d1-80b4-00c04fd430c8',
      'title': 'Обсуждение',
      'status': 'active',
      'created_at': '2026-05-01T12:00:00.000Z',
      'updated_at': '2026-05-02T15:30:00.000Z',
    };

    test('fromJson десериализует все поля', () {
      final c = ConversationModel.fromJson(validJson);

      expect(c.id, '550e8400-e29b-41d4-a716-446655440000');
      expect(c.projectId, '6ba7b810-9dad-11d1-80b4-00c04fd430c8');
      expect(c.title, 'Обсуждение');
      expect(c.status, 'active');
    });

    test('fromJson парсит DateTime из RFC3339', () {
      final c = ConversationModel.fromJson(validJson);

      expect(c.createdAt, isA<DateTime>());
      expect(c.updatedAt, isA<DateTime>());
      expect(c.createdAt.toUtc().year, 2026);
      expect(c.createdAt.toUtc().month, 5);
      expect(c.createdAt.toUtc().day, 1);
      expect(c.createdAt.toUtc().hour, 12);

      expect(c.updatedAt.toUtc().year, 2026);
      expect(c.updatedAt.toUtc().month, 5);
      expect(c.updatedAt.toUtc().day, 2);
      expect(c.updatedAt.toUtc().hour, 15);
      expect(c.updatedAt.toUtc().minute, 30);
    });

    test('toJson сериализует snake_case ключи', () {
      final c = ConversationModel(
        id: 'id-1',
        projectId: 'proj-1',
        title: 'T',
        status: 'archived',
        createdAt: DateTime.utc(2026, 1, 2, 3, 4, 5),
        updatedAt: DateTime.utc(2026, 1, 2, 3, 4, 6),
      );

      final json = c.toJson();

      expect(json['id'], 'id-1');
      expect(json['project_id'], 'proj-1');
      expect(json['title'], 'T');
      expect(json['status'], 'archived');
      expect(json['created_at'], isA<String>());
      expect(json['updated_at'], isA<String>());
    });

    test('copyWith не мутирует оригинал', () {
      final original = ConversationModel.fromJson(validJson);
      final next = original.copyWith(status: 'completed');

      expect(next.status, 'completed');
      expect(original.status, 'active');
    });
  });
}
