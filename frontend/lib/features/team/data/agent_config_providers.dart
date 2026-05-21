import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/team/data/project_secret_repository.dart';
import 'package:frontend/features/team/data/user_secret_repository.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'agent_config_providers.g.dart';

@Riverpod(keepAlive: true)
ProjectSecretRepository projectSecretRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ProjectSecretRepository(dio: dio);
}

@Riverpod(keepAlive: true)
UserSecretRepository userSecretRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return UserSecretRepository(dio: dio);
}

@riverpod
Future<List<SecretRefModel>> projectSecrets(Ref ref, String projectId) async {
  final repo = ref.watch(projectSecretRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('projectSecrets($projectId) disposed'));
  return repo.list(projectId, cancelToken: cancelToken);
}

@riverpod
Future<List<SecretRefModel>> userSecrets(Ref ref) async {
  final repo = ref.watch(userSecretRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('userSecrets disposed'));
  return repo.list(cancelToken: cancelToken);
}
