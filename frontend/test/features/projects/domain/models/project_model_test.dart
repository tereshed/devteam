import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';

void main() {
  group('ProjectModel', () {
    final validJson = <String, dynamic>{
      'id': 'project-123',
      'name': 'Test Project',
      'description': 'Test description',
      'git_provider': 'github',
      'git_url': 'https://github.com/test/repo',
      'git_default_branch': 'main',
      'git_credential': null,
      'vector_collection': 'DevTeam_Project_123',
      'tech_stack': <String, dynamic>{'backend': 'Go', 'frontend': 'Flutter'},
      'status': 'active',
      'settings': <String, dynamic>{},
      'created_at': '2026-04-27T10:00:00Z',
      'updated_at': '2026-04-27T10:30:00Z',
    };

    test('fromJson десериализует поля с правильными JSON-ключами', () {
      final project = ProjectModel.fromJson(validJson);

      expect(project.id, 'project-123');
      expect(project.name, 'Test Project');
      expect(project.gitProvider, 'github');
      expect(project.gitUrl, 'https://github.com/test/repo');
      expect(project.techStack, {'backend': 'Go', 'frontend': 'Flutter'});
      expect(project.status, 'active');
    });

    test('fromJson парсит DateTime из RFC3339-строк', () {
      final project = ProjectModel.fromJson(validJson);

      expect(project.createdAt, isA<DateTime>());
      expect(project.updatedAt, isA<DateTime>());
      expect(project.createdAt.year, 2026);
      expect(project.createdAt.month, 4);
    });

    test('toJson сериализует поля в snake_case JSON-ключи', () {
      final now = DateTime.now();
      final project = ProjectModel(
        id: 'p1',
        name: 'Test',
        description: 'Desc',
        gitProvider: 'github',
        gitUrl: 'https://github.com/test/repo',
        gitDefaultBranch: 'main',
        gitCredential: null,
        vectorCollection: 'vec-123',
        techStack: {},
        status: 'active',
        settings: {},
        createdAt: now,
        updatedAt: now,
      );

      final json = project.toJson();

      expect(json.containsKey('git_provider'), true);
      expect(json.containsKey('git_url'), true);
      expect(json.containsKey('created_at'), true);
      expect(json['git_provider'], 'github');
      expect(json['status'], 'active');
    });

    test('fromJson обрабатывает опциональное git_credential (присутствует)', () {
      final jsonWithCred = <String, dynamic>{...validJson};
      jsonWithCred['git_credential'] = <String, dynamic>{
        'id': 'cred-1',
        'provider': 'github',
        'auth_type': 'token',
        'label': 'My GitHub',
      };

      final project = ProjectModel.fromJson(jsonWithCred);
      expect(project.gitCredential, isNotNull);
      expect(project.gitCredential!.provider, 'github');
    });

    test('fromJson обрабатывает git_credential = null', () {
      final jsonWithNull = <String, dynamic>{...validJson};
      jsonWithNull['git_credential'] = null;

      final project = ProjectModel.fromJson(jsonWithNull);
      expect(project.gitCredential, isNull);
    });

    test('fromJson не падает если git_credential отсутствует в JSON (omitempty)',
        () {
      // Backend отправляет omitempty: если null/не установлен → ключ пропадает
      final jsonWithoutCred = <String, dynamic>{...validJson}
        ..remove('git_credential');

      final project = ProjectModel.fromJson(jsonWithoutCred);
      expect(project.gitCredential, isNull);
    });

    test('copyWith создаёт новый экземпляр без изменения оригинала', () {
      final original = ProjectModel.fromJson(validJson);
      final updated = original.copyWith(status: 'paused');

      expect(updated.status, 'paused');
      expect(original.status, 'active');
    });

    test('fromJson обрабатывает tech_stack = null (от gorm с пустой JSON)',
        () {
      final jsonWithNullTechStack = <String, dynamic>{...validJson};
      jsonWithNullTechStack['tech_stack'] = null;

      final project = ProjectModel.fromJson(jsonWithNullTechStack);
      expect(project.techStack, <String, dynamic>{});
    });

    test('fromJson обрабатывает settings = null (от gorm с пустой JSON)', () {
      final jsonWithNullSettings = <String, dynamic>{...validJson};
      jsonWithNullSettings['settings'] = null;

      final project = ProjectModel.fromJson(jsonWithNullSettings);
      expect(project.settings, <String, dynamic>{});
    });
  });

  group('GitCredentialModel', () {
    final validJson = <String, dynamic>{
      'id': 'cred-1',
      'provider': 'github',
      'auth_type': 'token',
      'label': 'My GitHub PAT',
    };

    test('fromJson десериализует все поля', () {
      final credential = GitCredentialModel.fromJson(validJson);

      expect(credential.id, 'cred-1');
      expect(credential.provider, 'github');
      expect(credential.authType, 'token');
      expect(credential.label, 'My GitHub PAT');
    });

    test('toJson сериализует в snake_case (auth_type)', () {
      const credential = GitCredentialModel(
        id: 'cred-1',
        provider: 'gitlab',
        authType: 'ssh_key',
        label: 'GitLab SSH',
      );

      final json = credential.toJson();

      expect(json.containsKey('auth_type'), true);
      expect(json['auth_type'], 'ssh_key');
      expect(json['provider'], 'gitlab');
    });
  });
}
