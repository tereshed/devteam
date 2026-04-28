import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';

void main() {
  group('TeamModel', () {
    final agentJson = <String, dynamic>{
      'id': 'agent-1',
      'name': 'DevAgent',
      'role': 'developer',
      'model': 'claude-opus-4-7',
      'prompt_name': 'Developer',
      'code_backend': 'claude-code',
      'is_active': true,
    };

    final validJson = <String, dynamic>{
      'id': 'team-123',
      'name': 'Dev Team',
      'project_id': 'project-123',
      'type': 'development',
      'agents': [agentJson],
      'created_at': '2026-04-27T09:00:00Z',
      'updated_at': '2026-04-27T09:15:00Z',
    };

    test('fromJson десериализует с project_id из snake_case', () {
      final team = TeamModel.fromJson(validJson);

      expect(team.id, 'team-123');
      expect(team.projectId, 'project-123');
      expect(team.name, 'Dev Team');
      expect(team.type, 'development');
      expect(team.agents, isNotEmpty);
    });

    test('fromJson конвертирует agents в список AgentModel', () {
      final team = TeamModel.fromJson(validJson);

      expect(team.agents.length, 1);
      expect(team.agents[0], isA<AgentModel>());
      expect(team.agents[0].name, 'DevAgent');
    });

    test('fromJson парсит DateTime из RFC3339-строк', () {
      final team = TeamModel.fromJson(validJson);

      expect(team.createdAt, isA<DateTime>());
      expect(team.updatedAt, isA<DateTime>());
      expect(team.createdAt.year, 2026);
      expect(team.createdAt.month, 4);
    });

    test('toJson сериализует в snake_case (project_id, created_at, updated_at)',
        () {
      final now = DateTime.now();
      final team = TeamModel(
        id: 't1',
        name: 'Test Team',
        projectId: 'p1',
        type: 'development',
        agents: [],
        createdAt: now,
        updatedAt: now,
      );

      final json = team.toJson();

      expect(json.containsKey('project_id'), true);
      expect(json.containsKey('created_at'), true);
      expect(json.containsKey('updated_at'), true);
      expect(json['project_id'], 'p1');
      expect(json['name'], 'Test Team');
    });

    test('copyWith создаёт новый экземпляр без изменения оригинала', () {
      final original = TeamModel.fromJson(validJson);
      final updated = original.copyWith(name: 'Updated Team');

      expect(updated.name, 'Updated Team');
      expect(original.name, 'Dev Team');
      expect(updated.agents.length, original.agents.length); // не изменилось
    });

    test('fromJson обрабатывает несколько агентов', () {
      final multiAgentJson = <String, dynamic>{
        ...validJson,
        'agents': [
          agentJson,
          <String, dynamic>{
            'id': 'agent-2',
            'name': 'ReviewerAgent',
            'role': 'reviewer',
            'model': 'claude-sonnet-4-6',
            'prompt_name': 'Reviewer',
            'code_backend': null,
            'is_active': true,
          },
        ],
      };

      final team = TeamModel.fromJson(multiAgentJson);

      expect(team.agents.length, 2);
      expect(team.agents[0].role, 'developer');
      expect(team.agents[1].role, 'reviewer');
      expect(team.agents[1].codeBackend, isNull);
    });

    test('fromJson обрабатывает agents = null как пустой список', () {
      final jsonWithNullAgents = <String, dynamic>{...validJson};
      jsonWithNullAgents['agents'] = null;

      final team = TeamModel.fromJson(jsonWithNullAgents);
      expect(team.agents, <AgentModel>[]);
    });
  });
}
