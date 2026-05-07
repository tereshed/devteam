// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';

void main() {
  group('TaskListResponse', () {
    test('fromJson: полный JSON, total как int', () {
      final json = <String, dynamic>{
        'tasks': [
          <String, dynamic>{
            'id': '11111111-1111-1111-1111-111111111111',
            'project_id': '22222222-2222-2222-2222-222222222222',
            'title': 'A',
            'status': kTaskStatusPending,
            'priority': 'medium',
            'created_by_type': kCreatedByTypeUser,
            'created_by_id': '33333333-3333-3333-3333-333333333333',
            'created_at': '2026-05-07T00:00:00.000Z',
            'updated_at': '2026-05-07T00:00:00.000Z',
          },
        ],
        'total': 100,
        'limit': 20,
        'offset': 0,
      };

      final r = TaskListResponse.fromJson(json);

      expect(r.tasks, hasLength(1));
      expect(r.tasks.single.title, 'A');
      expect(r.total, 100);
      expect(r.limit, 20);
      expect(r.offset, 0);
      expect(json.containsKey('has_next'), isFalse);
    });

    test('нет поля has_next в типе', () {
      final r = TaskListResponse.fromJson(<String, dynamic>{
        'tasks': <Object>[],
        'total': 0,
        'limit': 10,
        'offset': 0,
      });
      expect(r.tasks, isEmpty);
    });

    test('без ключа tasks — @Default даёт пустой список', () {
      final r = TaskListResponse.fromJson(<String, dynamic>{
        'total': 0,
        'limit': 10,
        'offset': 0,
      });
      expect(r.tasks, isEmpty);
    });
  });

  group('TaskMessageListResponse', () {
    test('fromJson: вложенные messages и пагинация', () {
      final json = <String, dynamic>{
        'messages': [
          <String, dynamic>{
            'id': 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
            'task_id': 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
            'sender_type': kSenderTypeAgent,
            'sender_id': 'cccccccc-cccc-cccc-cccc-cccccccccccc',
            'content': 'M',
            'message_type': kMessageTypeResult,
            'metadata': <String, dynamic>{},
            'created_at': '2026-05-07T12:00:00.000Z',
          },
        ],
        'total': 50,
        'limit': 10,
        'offset': 10,
      };

      final r = TaskMessageListResponse.fromJson(json);

      expect(r.messages, hasLength(1));
      expect(r.messages.single.content, 'M');
      expect(r.total, 50);
      expect(r.limit, 10);
      expect(r.offset, 10);
    });

    test('пустой messages и без has_next', () {
      final r = TaskMessageListResponse.fromJson(<String, dynamic>{
        'messages': <Object>[],
        'total': 0,
        'limit': 20,
        'offset': 0,
      });
      expect(r.messages, isEmpty);
    });

    test('без ключа messages — @Default даёт пустой список', () {
      final r = TaskMessageListResponse.fromJson(<String, dynamic>{
        'total': 1,
        'limit': 10,
        'offset': 0,
      });
      expect(r.messages, isEmpty);
    });
  });
}
