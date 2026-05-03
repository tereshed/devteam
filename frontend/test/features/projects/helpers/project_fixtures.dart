import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';

/// Имена проектов только для виджет-тестов дашборда (данные [ProjectModel.name], не строки ARB).
const String kTestDashboardProjectNameFixtureAlpha = 'Fixture Alpha';
const String kTestDashboardProjectNameAfterLoading = 'After loading';
const String kTestDashboardProjectNameAfterRetry = 'After retry';
const String kTestDashboardProjectNamePopped = 'Popped Project';

/// Второй UUID для сценария смены `:id` в URL дашборда (виджет-тесты).
const String kTestProjectUuidNavB = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';

/// Имена для теста смены `projectId` в URL (данные модели, не ARB).
const String kTestDashboardNavProjectNameA = 'NavProjA';
const String kTestDashboardNavProjectNameB = 'NavProjB';

ProjectModel makeProject({
  String id = 'proj-1',
  String name = 'Test Project',
  String description = 'A test project',
  String status = 'active',
  String gitProvider = 'github',
}) =>
    ProjectModel(
      id: id,
      name: name,
      description: description,
      gitProvider: gitProvider,
      gitUrl: 'https://github.com/user/repo.git',
      gitDefaultBranch: 'main',
      vectorCollection: 'test_col',
      status: status,
      createdAt: DateTime(2026, 1, 1),
      updatedAt: DateTime(2026, 4, 1),
    );

ProjectListResponse makeResponse(List<ProjectModel> projects) =>
    ProjectListResponse(
      projects: projects,
      total: projects.length,
      limit: 50,
      offset: 0,
    );
