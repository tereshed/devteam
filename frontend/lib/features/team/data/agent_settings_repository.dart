import 'package:dio/dio.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

/// Sprint 15.29 — HTTP-слой per-agent settings (15.23: /agents/:id/settings).
class AgentSettingsRepository {
  AgentSettingsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  /// GET /agents/:id/settings — текущие настройки агента.
  Future<AgentSettingsModel> get(String agentID) async {
    final resp =
        await _dio.get<Map<String, dynamic>>('/agents/$agentID/settings');
    return AgentSettingsModel.fromJson(resp.data!);
  }

  /// PUT /agents/:id/settings — частичное обновление.
  ///
  /// Любое из полей может быть опущено; пустой Map<String, dynamic> на стороне UI
  /// означает «не передавать» (репозиторий проверяет null).
  Future<AgentSettingsModel> update(
    String agentID, {
    String? llmProviderID,
    bool clearLLMProvider = false,
    String? codeBackend,
    Map<String, dynamic>? codeBackendSettings,
    Map<String, dynamic>? sandboxPermissions,
  }) async {
    final body = <String, dynamic>{
      if (llmProviderID != null) 'llm_provider_id': llmProviderID,
      if (clearLLMProvider) 'clear_llm_provider': true,
      if (codeBackend != null) 'code_backend': codeBackend,
      if (codeBackendSettings != null)
        'code_backend_settings': codeBackendSettings,
      if (sandboxPermissions != null)
        'sandbox_permissions': sandboxPermissions,
    };
    final resp = await _dio.put<Map<String, dynamic>>(
      '/agents/$agentID/settings',
      data: body,
    );
    return AgentSettingsModel.fromJson(resp.data!);
  }
}
