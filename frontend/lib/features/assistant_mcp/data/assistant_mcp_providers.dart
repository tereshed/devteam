import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/assistant_mcp/data/assistant_mcp_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_mcp_providers.g.dart';

@Riverpod(keepAlive: true)
AssistantMcpRepository assistantMcpRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return AssistantMcpRepository(dio: dio);
}
