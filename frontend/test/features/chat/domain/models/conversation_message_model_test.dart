// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/domain/models.dart';

void main() {
  group('ConversationMessageModel', () {
    test('conversationMessageRoles совпадает с backend ConversationRole', () {
      expect(
        conversationMessageRoles,
        equals(<String>['user', 'assistant', 'system']),
      );
    });

    Map<String, dynamic> baseJson({
      bool includeMetadata = true,
      Object? metadataValue,
      bool includeLinked = true,
      Object? linkedValue,
    }) {
      final m = <String, dynamic>{
        'id': 'msg-11111111-1111-1111-1111-111111111111',
        'conversation_id': 'conv-22222222-2222-2222-2222-222222222222',
        'role': 'user',
        'content': 'Привет',
        'created_at': '2026-05-03T10:00:00.000Z',
      };
      if (includeLinked) {
        m['linked_task_ids'] = linkedValue ?? <String>[];
      }
      if (includeMetadata) {
        m['metadata'] = metadataValue;
      }
      return m;
    }

    test('metadata: отсутствие ключа → null (типичный POST SendMessage)', () {
      final json = baseJson(includeMetadata: false);
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.metadata, isNull);
    });

    test('metadata: пустой объект {} (типичный GET из БД)', () {
      final json = baseJson(metadataValue: <String, dynamic>{});
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.metadata, isNotNull);
      expect(msg.metadata, isEmpty);
    });

    test('metadata: непустой объект', () {
      final json = baseJson(metadataValue: <String, dynamic>{'tokens': 42});
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.metadata!['tokens'], 42);
    });

    test('metadata: is_streaming без streaming → поднимается в streaming', () {
      final json = baseJson(
        metadataValue: <String, dynamic>{'is_streaming': true},
      );
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.metadata!['streaming'], isTrue);
      expect(msg.metadata!['is_streaming'], isTrue);
    });

    test('linked_task_ids: ключ отсутствует → пустой список', () {
      final json = baseJson(includeLinked: false);
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.linkedTaskIds, isEmpty);
    });

    test('linked_task_ids: [] → пустой список', () {
      final json = baseJson(linkedValue: <String>[]);
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.linkedTaskIds, isEmpty);
    });

    test('linked_task_ids: два UUID', () {
      const a = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
      const b = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
      final json = baseJson(linkedValue: <String>[a, b]);
      final msg = ConversationMessageModel.fromJson(json);

      expect(msg.linkedTaskIds, [a, b]);
    });

    test('toJson кладёт metadata: null; round-trip fromJson(toJson) == исходник', () {
      final created = DateTime.utc(2026, 5, 3, 10);
      final original = ConversationMessageModel(
        id: 'msg-11111111-1111-1111-1111-111111111111',
        conversationId: 'conv-22222222-2222-2222-2222-222222222222',
        role: 'user',
        content: 'Привет',
        linkedTaskIds: const [],
        metadata: null,
        createdAt: created,
      );

      final json = original.toJson();
      expect(json['metadata'], isNull);

      final roundTrip = ConversationMessageModel.fromJson(json);
      expect(roundTrip, original);
    });

    test('round-trip с metadata и linked_task_ids', () {
      final created = DateTime.utc(2026, 5, 3, 11, 22, 33);
      final original = ConversationMessageModel(
        id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        conversationId: 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        role: 'assistant',
        content: 'Ответ',
        linkedTaskIds: const [
          'cccccccc-cccc-cccc-cccc-cccccccccccc',
        ],
        metadata: <String, dynamic>{'tokens': 7},
        createdAt: created,
      );

      final roundTrip =
          ConversationMessageModel.fromJson(original.toJson());
      expect(roundTrip, original);
    });
  });
}
