import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/role_prompts_repository.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'role_prompts_providers.g.dart';

@Riverpod(keepAlive: true)
RolePromptsRepository rolePromptsRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return RolePromptsRepository(dio: dio);
}

@riverpod
Future<List<AgentRolePromptModel>> rolePromptsList(Ref ref) async {
  final repo = ref.watch(rolePromptsRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('rolePromptsList disposed'));
  return repo.list(cancelToken: cancelToken);
}
