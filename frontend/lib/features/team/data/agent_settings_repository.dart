import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/team/domain/agent_settings_exceptions.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

/// Sprint 15.29 / 15.B (F9) — HTTP-слой per-agent settings.
/// Использует канонический [mapDioExceptionForRepository].
class AgentSettingsRepository {
  AgentSettingsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          AgentSettingsCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          AgentSettingsApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => AgentSettingsForbiddenException(msg,
          originalError: err, apiErrorCode: code),
      on404: (msg, err, code) => AgentSettingsNotFoundException(msg,
          originalError: err, apiErrorCode: code),
      on409: (msg, err, code) => AgentSettingsConflictException(msg,
          originalError: err, apiErrorCode: code),
      onOtherHttp: (msg, err, code, status) => AgentSettingsApiException(msg,
          statusCode: status, originalError: err, apiErrorCode: code),
    );
  }

  Future<AgentSettingsModel> get(
    String agentID, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/agents/$agentID/settings',
        cancelToken: cancelToken,
      );
      return AgentSettingsModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  // Sprint 15.e2e: параметры llm_provider_id / clear_llm_provider удалены вместе
  // с колонкой agents.llm_provider_id (миграция 029). provider_kind агента
  // меняется через teamRepository.patchAgent (поле provider_kind в PATCH-теле).
  Future<AgentSettingsModel> update(
    String agentID, {
    String? codeBackend,
    Map<String, dynamic>? codeBackendSettings,
    Map<String, dynamic>? sandboxPermissions,
    CancelToken? cancelToken,
  }) async {
    final body = <String, dynamic>{
      if (codeBackend != null) 'code_backend': codeBackend,
      if (codeBackendSettings != null)
        'code_backend_settings': codeBackendSettings,
      if (sandboxPermissions != null)
        'sandbox_permissions': sandboxPermissions,
    };
    try {
      final resp = await _dio.put<Map<String, dynamic>>(
        '/agents/$agentID/settings',
        data: body,
        cancelToken: cancelToken,
      );
      return AgentSettingsModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
