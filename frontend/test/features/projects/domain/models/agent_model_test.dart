import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';

void main() {
  group('AgentModel', () {
    final validJson = <String, dynamic>{
      'id': 'agent-1',
      'name': 'DevAgent',
      'role': 'developer',
      'model': 'claude-opus-4-7',
      'prompt_name': 'Developer',
      'code_backend': 'claude-code',
      'is_active': true,
    };

    test('fromJson десериализует все поля с правильными JSON-ключами', () {
      final agent = AgentModel.fromJson(validJson);

      expect(agent.id, 'agent-1');
      expect(agent.name, 'DevAgent');
      expect(agent.role, 'developer');
      expect(agent.model, 'claude-opus-4-7');
      expect(agent.promptName, 'Developer');
      expect(agent.codeBackend, 'claude-code');
      expect(agent.isActive, true);
    });

    test('fromJson обрабатывает nullable поля (model, prompt_name, code_backend)',
        () {
      final jsonWithNulls = <String, dynamic>{
        'id': 'agent-2',
        'name': 'ReviewAgent',
        'role': 'reviewer',
        'model': null,
        'prompt_name': null,
        'code_backend': null,
        'is_active': true,
      };

      final agent = AgentModel.fromJson(jsonWithNulls);

      expect(agent.model, isNull);
      expect(agent.promptName, isNull);
      expect(agent.codeBackend, isNull);
    });

    test('fromJson преобразует is_active в isActive', () {
      final json = <String, dynamic>{...validJson};
      json['is_active'] = false;

      final agent = AgentModel.fromJson(json);
      expect(agent.isActive, false);
    });

    test('toJson сериализует в snake_case (prompt_name, code_backend, is_active)',
        () {
      const agent = AgentModel(
        id: 'a1',
        name: 'Agent',
        role: 'developer',
        model: 'claude-opus-4-7',
        promptName: 'Dev',
        codeBackend: 'claude-code',
        isActive: true,
      );

      final json = agent.toJson();

      expect(json.containsKey('prompt_name'), true);
      expect(json.containsKey('code_backend'), true);
      expect(json.containsKey('is_active'), true);
      expect(json['prompt_name'], 'Dev');
      expect(json['is_active'], true);
    });

    test('copyWith создаёт новый экземпляр без изменения оригинала', () {
      final original = AgentModel.fromJson(validJson);
      final updated = original.copyWith(isActive: false);

      expect(updated.isActive, false);
      expect(original.isActive, true);
      expect(updated.name, original.name); // не изменилось
    });

    test('fromJson обрабатывает omitted nullable поля', () {
      // Некоторые null поля могут быть отсутствующими в JSON (omitempty)
      final jsonWithoutOptional = <String, dynamic>{
        'id': 'agent-3',
        'name': 'WorkerAgent',
        'role': 'worker',
        'is_active': true,
        // model, prompt_name, code_backend отсутствуют
      };

      final agent = AgentModel.fromJson(jsonWithoutOptional);

      expect(agent.id, 'agent-3');
      expect(agent.model, isNull);
      expect(agent.promptName, isNull);
      expect(agent.codeBackend, isNull);
      expect(agent.isActive, true);
    });

    test('fromJson обрабатывает различные role значения', () {
      const roles = [
        'orchestrator',
        'planner',
        'developer',
        'reviewer',
        'tester',
        'worker',
        'supervisor',
        'devops',
      ];

      for (final role in roles) {
        final json = <String, dynamic>{
          'id': 'agent-test',
          'name': 'Test Agent',
          'role': role,
          'is_active': true,
        };

        final agent = AgentModel.fromJson(json);
        expect(agent.role, role);
      }
    });

    test('fromJson обрабатывает различные code_backend значения', () {
      const backends = ['claude-code', 'aider', 'custom'];

      for (final backend in backends) {
        final json = <String, dynamic>{
          ...validJson,
          'code_backend': backend,
        };

        final agent = AgentModel.fromJson(json);
        expect(agent.codeBackend, backend);
      }
    });
  });
}
