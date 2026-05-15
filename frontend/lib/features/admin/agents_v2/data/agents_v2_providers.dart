import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_repository.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';

final agentsV2RepositoryProvider = Provider<AgentsV2Repository>((ref) {
  final dio = ref.watch(dioClientProvider);
  return AgentsV2Repository(dio: dio);
});

/// Список агентов (без фильтров — фильтрация на клиенте в списке).
final agentsV2ListProvider = FutureProvider.autoDispose<AgentV2Page>((ref) {
  final repo = ref.watch(agentsV2RepositoryProvider);
  return repo.list(limit: 200);
});

/// Полная запись агента (с system_prompt).
final agentV2DetailProvider =
    FutureProvider.autoDispose.family<AgentV2, String>((ref, id) {
  final repo = ref.watch(agentsV2RepositoryProvider);
  return repo.get(id);
});
