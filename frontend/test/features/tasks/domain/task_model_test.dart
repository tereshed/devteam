// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';

void main() {
  group('TaskModel', () {
    test('fromJson: полный ответ GET /tasks/:id', () {
      final json = <String, dynamic>{
        'id': '11111111-1111-1111-1111-111111111111',
        'project_id': '22222222-2222-2222-2222-222222222222',
        'title': 'Задача',
        'description': 'Описание',
        'status': kTaskStatusInProgress,
        'priority': 'high',
        'created_by_type': kCreatedByTypeUser,
        'created_by_id': '33333333-3333-3333-3333-333333333333',
        'context': <String, dynamic>{'k': 'v'},
        'artifacts': <String, dynamic>{'diff': '--- a\n+++ b'},
        'assigned_agent': <String, dynamic>{
          'id': '44444444-4444-4444-4444-444444444444',
          'name': 'Dev',
          'role': 'developer',
        },
        'sub_tasks': <Map<String, dynamic>>[
          <String, dynamic>{
            'id': '55555555-5555-5555-5555-555555555555',
            'title': 'Подзадача',
            'status': kTaskStatusPending,
            'priority': 'medium',
          },
        ],
        'branch_name': 'feature/foo',
        'result': 'Готово',
        'error_message': null,
        'started_at': '2026-05-01T10:00:00.000Z',
        'completed_at': null,
        'created_at': '2026-05-01T09:00:00.000Z',
        'updated_at': '2026-05-01T11:00:00.000Z',
      };

      final m = TaskModel.fromJson(json);

      expect(m.id, '11111111-1111-1111-1111-111111111111');
      expect(m.projectId, '22222222-2222-2222-2222-222222222222');
      expect(m.title, 'Задача');
      expect(m.status, kTaskStatusInProgress);
      expect(m.assignedAgent?.name, 'Dev');
      expect(m.assignedAgent?.role, 'developer');
      expect(m.context, <String, dynamic>{'k': 'v'});
      expect(m.artifacts, containsPair('diff', '--- a\n+++ b'));
      expect(m.subTasks, hasLength(1));
      expect(m.subTasks.single.title, 'Подзадача');
      expect(m.branchName, 'feature/foo');
      expect(m.result, 'Готово');
      expect(m.messageCount, isNull);
      expect(m.startedAt?.isUtc, isTrue);
      expect(m.createdAt.isUtc, isTrue);
    });

    test('message_count: отсутствует ключ → null', () {
      final json = _minimalTaskJson();
      expect(TaskModel.fromJson(json).messageCount, isNull);
    });

    test('message_count: null в JSON → null', () {
      final json = _minimalTaskJson()..['message_count'] = null;
      expect(TaskModel.fromJson(json).messageCount, isNull);
    });

    test('message_count: число парсится', () {
      final json = _minimalTaskJson()..['message_count'] = 42;
      expect(TaskModel.fromJson(json).messageCount, 42);
    });

    test('fromJson: все omitempty-поля отсутствуют — не падает', () {
      final json = <String, dynamic>{
        'id': '11111111-1111-1111-1111-111111111111',
        'project_id': '22222222-2222-2222-2222-222222222222',
        'title': 'T',
        'description': '',
        'status': kTaskStatusPending,
        'priority': 'medium',
        'created_by_type': kCreatedByTypeAgent,
        'created_by_id': '33333333-3333-3333-3333-333333333333',
        'created_at': '2026-05-07T00:00:00.000Z',
        'updated_at': '2026-05-07T00:00:00.000Z',
      };

      final m = TaskModel.fromJson(json);

      expect(m.parentTaskId, isNull);
      expect(m.assignedAgent, isNull);
      expect(m.result, isNull);
      expect(m.branchName, isNull);
      expect(m.errorMessage, isNull);
      expect(m.messageCount, isNull);
      expect(m.startedAt, isNull);
      expect(m.completedAt, isNull);
      expect(m.subTasks, isEmpty);
      expect(m.context, isEmpty);
      expect(m.artifacts, isEmpty);
    });

    test('sub_tasks: пустой массив', () {
      final json = _minimalTaskJson()..['sub_tasks'] = <Object>[];
      expect(TaskModel.fromJson(json).subTasks, isEmpty);
    });

    test('sub_tasks: без ключа → пустой список', () {
      final json = _minimalTaskJson();
      expect(TaskModel.fromJson(json).subTasks, isEmpty);
    });

    test('roundtrip fromJson → toJson → fromJson и snake_case в toJson', () {
      final original = TaskModel.fromJson(<String, dynamic>{
        'id': '11111111-1111-1111-1111-111111111111',
        'project_id': '22222222-2222-2222-2222-222222222222',
        'title': 'T',
        'description': 'D',
        'status': kTaskStatusCompleted,
        'priority': 'low',
        'created_by_type': kCreatedByTypeUser,
        'created_by_id': '33333333-3333-3333-3333-333333333333',
        'context': <String, dynamic>{'a': 1},
        'artifacts': <String, dynamic>{},
        'sub_tasks': <Object>[],
        'created_at': '2026-05-07T12:00:00.000Z',
        'updated_at': '2026-05-07T12:30:00.000Z',
      });

      final map = original.toJson();
      expect(map.containsKey('project_id'), isTrue);
      expect(map.containsKey('created_by_type'), isTrue);
      expect(map.containsKey('sub_tasks'), isTrue);
      expect(map.containsKey('created_at'), isTrue);
      expect(map.containsKey('id'), isTrue);

      final again = TaskModel.fromJson(map);
      expect(again, equals(original));
    });
  });
}

Map<String, dynamic> _minimalTaskJson() => <String, dynamic>{
      'id': '11111111-1111-1111-1111-111111111111',
      'project_id': '22222222-2222-2222-2222-222222222222',
      'title': 'T',
      'description': '',
      'status': kTaskStatusPending,
      'priority': 'medium',
      'created_by_type': kCreatedByTypeUser,
      'created_by_id': '33333333-3333-3333-3333-333333333333',
      'created_at': '2026-05-07T00:00:00.000Z',
      'updated_at': '2026-05-07T00:00:00.000Z',
    };
