import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/onboarding/data/my_agents_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'my_agents_providers.g.dart';

@Riverpod(keepAlive: true)
MyAgentsRepository myAgentsRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return MyAgentsRepository(dio: dio);
}

@riverpod
Future<AgentV2Page> myAgentsList(Ref ref) {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);
  return ref.watch(myAgentsRepositoryProvider).list(cancelToken: cancelToken);
}
