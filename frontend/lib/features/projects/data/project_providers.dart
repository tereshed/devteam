import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'project_providers.g.dart';

@Riverpod(keepAlive: true)
ProjectRepository projectRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ProjectRepository(dio: dio);
}

@riverpod
Future<ProjectListResponse> projectList(
  Ref ref, {
  ProjectListFilter? filter,
  int limit = 20,
  int offset = 0,
}) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);

  return ref.watch(projectRepositoryProvider).listProjects(
        filter: filter,
        limit: limit,
        offset: offset,
        cancelToken: cancelToken,
      );
}

@riverpod
Future<ProjectModel> project(Ref ref, String id) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);

  return ref.watch(projectRepositoryProvider).getProject(
        id,
        cancelToken: cancelToken,
      );
}
