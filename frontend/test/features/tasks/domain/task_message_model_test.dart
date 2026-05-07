// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_message_model.dart';

void main() {
  group('TaskMessageModel', () {
    test('fromJson: metadata пустой объект', () {
      final json = <String, dynamic>{
        'id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'task_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'sender_type': kSenderTypeAgent,
        'sender_id': 'cccccccc-cccc-cccc-cccc-cccccccccccc',
        'content': 'Текст',
        'message_type': kMessageTypeResult,
        'metadata': <String, dynamic>{},
        'created_at': '2026-05-07T09:00:00.000Z',
      };

      final m = TaskMessageModel.fromJson(json);
      expect(m.metadata, isEmpty);
    });

    test('fromJson: без ключа metadata — @Default даёт пустую карту', () {
      final json = <String, dynamic>{
        'id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'task_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'sender_type': kSenderTypeUser,
        'sender_id': 'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'content': 'X',
        'message_type': kMessageTypeInstruction,
        'created_at': '2026-05-07T09:00:00.000Z',
      };

      final m = TaskMessageModel.fromJson(json);
      expect(m.metadata, isEmpty);
    });

    test('fromJson: metadata с произвольными ключами', () {
      final json = <String, dynamic>{
        'id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'task_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'sender_type': kSenderTypeUser,
        'sender_id': 'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'content': 'C',
        'message_type': kMessageTypeInstruction,
        'metadata': <String, dynamic>{
          'tokens_used': 100,
          'nested': <String, dynamic>{'x': true},
        },
        'created_at': '2026-05-07T09:00:00.000Z',
      };

      final m = TaskMessageModel.fromJson(json);
      expect(m.metadata['tokens_used'], 100);
      expect(m.metadata['nested'], isA<Map>());
    });

    test('roundtrip fromJson → toJson → fromJson и snake_case', () {
      final original = TaskMessageModel.fromJson(<String, dynamic>{
        'id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        'task_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        'sender_type': kSenderTypeUser,
        'sender_id': 'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'content': 'Hello',
        'message_type': kMessageTypeFeedback,
        'metadata': <String, dynamic>{'k': 1},
        'created_at': '2026-05-07T10:00:00.000Z',
      });

      final map = original.toJson();
      expect(map['task_id'], isNotNull);
      expect(map['sender_type'], kSenderTypeUser);
      expect(map['message_type'], kMessageTypeFeedback);

      final again = TaskMessageModel.fromJson(map);
      expect(again, equals(original));
    });
  });
}
