import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';

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
