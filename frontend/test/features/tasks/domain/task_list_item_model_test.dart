// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/domain/models/task_list_item_model.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';

void main() {
  group('TaskListItemModel', () {
    test('fromJson: без assigned_agent → null', () {
      final json = <String, dynamic>{
        'id': '11111111-1111-1111-1111-111111111111',
        'project_id': '22222222-2222-2222-2222-222222222222',
        'title': 'Строка списка',
        'status': kTaskStatusPending,
        'priority': 'medium',
        'created_by_type': kCreatedByTypeUser,
        'created_by_id': '33333333-3333-3333-3333-333333333333',
        'created_at': '2026-05-07T08:00:00.000Z',
        'updated_at': '2026-05-07T08:00:00.000Z',
      };

      final m = TaskListItemModel.fromJson(json);
      expect(m.assignedAgent, isNull);
      expect(m.title, 'Строка списка');
    });

    test('fromJson: с assigned_agent', () {
      final json = <String, dynamic>{
        'id': '11111111-1111-1111-1111-111111111111',
        'project_id': '22222222-2222-2222-2222-222222222222',
        'title': 'T',
        'status': kTaskStatusInProgress,
        'priority': 'high',
        'assigned_agent': <String, dynamic>{
          'id': '44444444-4444-4444-4444-444444444444',
          'name': 'Agent',
          'role': 'reviewer',
        },
        'created_by_type': kCreatedByTypeAgent,
        'created_by_id': '33333333-3333-3333-3333-333333333333',
        'created_at': '2026-05-07T08:00:00.000Z',
        'updated_at': '2026-05-07T08:00:00.000Z',
      };

      final m = TaskListItemModel.fromJson(json);
      expect(m.assignedAgent?.id, '44444444-4444-4444-4444-444444444444');
      expect(m.assignedAgent?.role, 'reviewer');
    });
  });
}
