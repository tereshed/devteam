import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/utils/project_settings_update_patch.dart';

import '../../helpers/project_fixtures.dart';

void main() {
  test(
    'explicit clear tech stack: clear_tech_stack при baseline только с non-string',
    () {
      final baseline = makeProject(
        techStack: const {'version': 18},
      );
      final req = buildProjectSettingsUpdateRequest(
        baseline: baseline,
        gitProvider: baseline.gitProvider,
        gitUrl: baseline.gitUrl,
        gitDefaultBranch: baseline.gitDefaultBranch,
        vectorCollection: baseline.vectorCollection,
        techStackEditedNonEmptyKeys: const {},
        pendingRemoveGitCredential: false,
        explicitClearTechStack: true,
      );
      expect(req, isNotNull);
      expect(req!.clearTechStack, isTrue);
      expect(req.techStack, isNull);
    },
  );

  test(
    'смена git_provider на local при gitCredential → remove_git_credential и git_provider',
    () {
      final baseline = makeProject(
        gitProvider: 'github',
        gitCredential: const GitCredentialModel(
          id: 'cred-1',
          provider: 'github',
          authType: 'token',
          label: 'My token',
        ),
      );
      final req = buildProjectSettingsUpdateRequest(
        baseline: baseline,
        gitProvider: kLocalGitProvider,
        gitUrl: baseline.gitUrl,
        gitDefaultBranch: baseline.gitDefaultBranch,
        vectorCollection: baseline.vectorCollection,
        techStackEditedNonEmptyKeys: const {},
        pendingRemoveGitCredential: false,
        explicitClearTechStack: false,
      );
      expect(req, isNotNull);
      expect(req!.gitProvider, kLocalGitProvider);
      expect(req.removeGitCredential, isTrue);
    },
  );

  test(
    'частичное изменение tech_stack сохраняет non-string ключи baseline',
    () {
      final baseline = makeProject(
        techStack: {
          'version': 18,
          'lang': 'Go',
        },
      );
      final req = buildProjectSettingsUpdateRequest(
        baseline: baseline,
        gitProvider: baseline.gitProvider,
        gitUrl: baseline.gitUrl,
        gitDefaultBranch: baseline.gitDefaultBranch,
        vectorCollection: baseline.vectorCollection,
        techStackEditedNonEmptyKeys: const {'lang': 'Rust'},
        pendingRemoveGitCredential: false,
        explicitClearTechStack: false,
      );
      expect(req, isNotNull);
      expect(req!.techStack, {'version': 18, 'lang': 'Rust'});
      expect(req.clearTechStack, isNull);
    },
  );

  test('нет отличий от baseline → null', () {
    final baseline = makeProject();
    final req = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: baseline.gitProvider,
      gitUrl: baseline.gitUrl,
      gitDefaultBranch: baseline.gitDefaultBranch,
      vectorCollection: baseline.vectorCollection,
      techStackEditedNonEmptyKeys: const {},
      pendingRemoveGitCredential: false,
      explicitClearTechStack: false,
    );
    expect(req, isNull);
  });

  test(
    'ensureTechStackMutuallyExclusive: tech_stack + clear_tech_stack → StateError',
    () {
      expect(
        () => ensureTechStackMutuallyExclusive(
          techStack: const {'a': 'b'},
          clearTechStack: true,
        ),
        throwsStateError,
      );
    },
  );

  test('задание шаблона ветки → branch_name_template в патче', () {
    final baseline = makeProject();
    final req = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: baseline.gitProvider,
      gitUrl: baseline.gitUrl,
      gitDefaultBranch: baseline.gitDefaultBranch,
      vectorCollection: baseline.vectorCollection,
      techStackEditedNonEmptyKeys: const {},
      pendingRemoveGitCredential: false,
      explicitClearTechStack: false,
      branchNameTemplate: 'issue/{ticket}_{slug}',
    );
    expect(req, isNotNull);
    expect(req!.branchNameTemplate, 'issue/{ticket}_{slug}');
  });

  test('сброс шаблона ветки (пустая строка при заданном baseline) → "" в патче',
      () {
    final baseline =
        makeProject().copyWith(branchNameTemplate: 'issue/{ticket}_{slug}');
    final req = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: baseline.gitProvider,
      gitUrl: baseline.gitUrl,
      gitDefaultBranch: baseline.gitDefaultBranch,
      vectorCollection: baseline.vectorCollection,
      techStackEditedNonEmptyKeys: const {},
      pendingRemoveGitCredential: false,
      explicitClearTechStack: false,
      branchNameTemplate: '',
    );
    expect(req, isNotNull);
    expect(req!.branchNameTemplate, '');
  });

  test('переключение замка → branch_naming_locked в патче', () {
    final baseline = makeProject();
    final req = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: baseline.gitProvider,
      gitUrl: baseline.gitUrl,
      gitDefaultBranch: baseline.gitDefaultBranch,
      vectorCollection: baseline.vectorCollection,
      techStackEditedNonEmptyKeys: const {},
      pendingRemoveGitCredential: false,
      explicitClearTechStack: false,
      branchNamingLocked: true,
    );
    expect(req, isNotNull);
    expect(req!.branchNamingLocked, isTrue);
  });

  test('шаблон без изменений → не попадает в патч', () {
    final baseline =
        makeProject().copyWith(branchNameTemplate: 'task/{short_id}-{slug}');
    final req = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: baseline.gitProvider,
      gitUrl: baseline.gitUrl,
      gitDefaultBranch: baseline.gitDefaultBranch,
      vectorCollection: baseline.vectorCollection,
      techStackEditedNonEmptyKeys: const {},
      pendingRemoveGitCredential: false,
      explicitClearTechStack: false,
      branchNameTemplate: 'task/{short_id}-{slug}',
    );
    expect(req, isNull);
  });
}
