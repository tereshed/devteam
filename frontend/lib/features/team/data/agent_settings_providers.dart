import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/team/data/agent_settings_repository.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'agent_settings_providers.g.dart';

@Riverpod(keepAlive: true)
AgentSettingsRepository agentSettingsRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return AgentSettingsRepository(dio: dio);
}

/// Sprint 15.29 — настройки одного агента (cache key: agentID).
@riverpod
Future<AgentSettingsModel> agentSettings(Ref ref, String agentID) async {
  final repo = ref.watch(agentSettingsRepositoryProvider);
  return repo.get(agentID);
}
