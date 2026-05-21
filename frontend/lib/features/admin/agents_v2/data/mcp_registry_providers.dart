import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/mcp_registry_repository.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'mcp_registry_providers.g.dart';

@Riverpod(keepAlive: true)
MCPRegistryRepository mcpRegistryRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return MCPRegistryRepository(dio: dio);
}

@riverpod
Future<List<MCPServerRegistryModel>> mcpRegistryList(Ref ref) async {
  final repo = ref.watch(mcpRegistryRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('mcpRegistryList disposed'));
  return repo.list(cancelToken: cancelToken);
}
