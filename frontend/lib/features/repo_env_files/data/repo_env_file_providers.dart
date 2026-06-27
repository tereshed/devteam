import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/repo_env_files/data/repo_env_file_repository.dart';
import 'package:frontend/features/repo_env_files/domain/models/repo_env_file_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'repo_env_file_providers.g.dart';

@Riverpod(keepAlive: true)
RepoEnvFileRepository repoEnvFileRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return RepoEnvFileRepository(dio: dio);
}

/// Список env-файлов конкретного репозитория проекта (метаданные, без содержимого).
@riverpod
Future<List<RepoEnvFileModel>> repoEnvFiles(
  Ref ref,
  String projectId,
  String repoId,
) async {
  final repo = ref.watch(repoEnvFileRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('repoEnvFiles($projectId,$repoId) disposed'));
  return repo.list(projectId, repoId, cancelToken: cancelToken);
}
