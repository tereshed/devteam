// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/domain/requests.dart';

void main() {
  group('ConversationListResponse', () {
    test('fromJson: вложенные conversations и has_next', () {
      final json = <String, dynamic>{
        'conversations': [
          {
            'id': 'c1',
            'project_id': 'p1',
            'title': 'Чат 1',
            'status': 'active',
            'created_at': '2026-05-01T00:00:00.000Z',
            'updated_at': '2026-05-01T00:00:00.000Z',
          },
        ],
        'total': 100,
        'limit': 20,
        'offset': 0,
        'has_next': true,
      };

      final r = ConversationListResponse.fromJson(json);

      expect(r.conversations, hasLength(1));
      expect(r.conversations.single.id, 'c1');
      expect(r.conversations.single.projectId, 'p1');
      expect(r.total, 100);
      expect(r.limit, 20);
      expect(r.offset, 0);
      expect(r.hasNext, isTrue);
    });

    test('has_next false когда страница последняя', () {
      final json = <String, dynamic>{
        'conversations': <Object>[],
        'total': 10,
        'limit': 20,
        'offset': 0,
        'has_next': false,
      };

      final r = ConversationListResponse.fromJson(json);
      expect(r.hasNext, isFalse);
      expect(r.conversations, isEmpty);
    });
  });

  group('MessageListResponse', () {
    test('fromJson: вложенные messages и has_next', () {
      final json = <String, dynamic>{
        'messages': [
          {
            'id': 'm1',
            'conversation_id': 'c1',
            'role': 'assistant',
            'content': 'Ответ',
            'linked_task_ids': <String>[],
            'metadata': <String, dynamic>{},
            'created_at': '2026-05-03T12:00:00.000Z',
          },
        ],
        'total': 50,
        'limit': 10,
        'offset': 10,
        'has_next': true,
      };

      final r = MessageListResponse.fromJson(json);

      expect(r.messages, hasLength(1));
      expect(r.messages.single.role, 'assistant');
      expect(r.messages.single.content, 'Ответ');
      expect(r.messages.single.metadata, isNotNull);
      expect(r.total, 50);
      expect(r.limit, 10);
      expect(r.offset, 10);
      expect(r.hasNext, isTrue);
    });
  });
}
