import 'package:dio/dio.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';

/// Sprint 15.29 — HTTP-слой LLM-провайдеров.
///
/// CRUD-эндпоинты бэка ещё не добавлены публично (Sprint 15.10 пока даёт только service),
/// поэтому фронтовая часть использует REST-конвенцию `GET/POST/PUT/DELETE /llm-providers`.
/// При расхождении пути — обновить здесь после релиза backend handler'а.
class LLMProvidersRepository {
  LLMProvidersRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;
  static const _basePath = '/llm-providers';

  /// GET /llm-providers — список включённых/всех провайдеров.
  Future<List<LLMProviderModel>> list({bool onlyEnabled = false}) async {
    final resp = await _dio.get<dynamic>(
      _basePath,
      queryParameters: {if (onlyEnabled) 'only_enabled': 'true'},
    );
    final data = resp.data;
    if (data is! List) {
      throw DioException(
        requestOptions: resp.requestOptions,
        response: resp,
        message: 'Expected array response',
      );
    }
    return data
        .map((e) => LLMProviderModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// POST /llm-providers — создать.
  Future<LLMProviderModel> create({
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    String? credential,
    String defaultModel = '',
    bool enabled = true,
  }) async {
    final resp = await _dio.post<Map<String, dynamic>>(
      _basePath,
      data: <String, dynamic>{
        'name': name,
        'kind': kind,
        'base_url': baseURL,
        'auth_type': authType,
        if (credential != null) 'credential': credential,
        'default_model': defaultModel,
        'enabled': enabled,
      },
    );
    return LLMProviderModel.fromJson(resp.data!);
  }

  /// PUT /llm-providers/:id — обновить (credential null/'' — не менять).
  Future<LLMProviderModel> update({
    required String id,
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    String? credential,
    String defaultModel = '',
    bool enabled = true,
  }) async {
    final resp = await _dio.put<Map<String, dynamic>>(
      '$_basePath/$id',
      data: <String, dynamic>{
        'name': name,
        'kind': kind,
        'base_url': baseURL,
        'auth_type': authType,
        if (credential != null && credential.isNotEmpty)
          'credential': credential,
        'default_model': defaultModel,
        'enabled': enabled,
      },
    );
    return LLMProviderModel.fromJson(resp.data!);
  }

  /// DELETE /llm-providers/:id
  Future<void> delete(String id) async {
    await _dio.delete<dynamic>('$_basePath/$id');
  }

  /// POST /llm-providers/:id/health-check — пинг провайдера (Sprint 15.10).
  Future<void> healthCheck(String id) async {
    await _dio.post<dynamic>('$_basePath/$id/health-check');
  }

  /// POST /llm-providers/test-connection — проверка кредов перед сохранением.
  Future<void> testConnection({
    required String name,
    required String kind,
    String baseURL = '',
    String authType = 'api_key',
    required String credential,
    String defaultModel = '',
  }) async {
    await _dio.post<dynamic>(
      '$_basePath/test-connection',
      data: <String, dynamic>{
        'name': name,
        'kind': kind,
        'base_url': baseURL,
        'auth_type': authType,
        'credential': credential,
        'default_model': defaultModel,
      },
    );
  }
}
