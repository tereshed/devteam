import 'package:dio/dio.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';

class MyAgentsRepository {
  MyAgentsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Future<AgentV2Page> list({CancelToken? cancelToken}) async {
    final response = await _dio.get(
      '/me/agents',
      cancelToken: cancelToken,
    );
    final json = response.data as Map<String, dynamic>;
    return AgentV2Page.fromJson(json);
  }

  Future<AgentV2> update(
    String id, {
    String? model,
    String? providerKind,
    String? systemPrompt,
    Map<String, dynamic>? settings,
    CancelToken? cancelToken,
  }) async {
    final response = await _dio.put(
      '/me/agents/$id',
      data: {
        if (model != null) 'model': model,
        if (providerKind != null) 'provider_kind': providerKind,
        if (systemPrompt != null) 'system_prompt': systemPrompt,
        if (settings != null) 'settings': settings,
      },
      cancelToken: cancelToken,
    );
    final json = response.data as Map<String, dynamic>;
    return AgentV2.fromJson(json);
  }

  /// Мой агент-ассистент ([GET /me/assistant]); провижится на бэкенде при
  /// отсутствии. Полная запись с `system_prompt`.
  Future<AgentV2> getAssistant({CancelToken? cancelToken}) async {
    final response = await _dio.get(
      '/me/assistant',
      cancelToken: cancelToken,
    );
    final json = response.data as Map<String, dynamic>;
    return AgentV2.fromJson(json);
  }
}
