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

/// env-файл конкретного репозитория проекта (null — не настроен).
@riverpod
Future<RepoEnvFileModel?> repoEnvFile(
  Ref ref,
  String projectId,
  String repoId,
) async {
  final repo = ref.watch(repoEnvFileRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('repoEnvFile($projectId,$repoId) disposed'));
  return repo.get(projectId, repoId, cancelToken: cancelToken);
}
