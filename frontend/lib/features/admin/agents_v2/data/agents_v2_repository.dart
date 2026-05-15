import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_exceptions.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';

class AgentsV2Repository {
  AgentsV2Repository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonMap(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) =>
            throw AgentV2ApiException(msg, statusCode: code),
      );

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          AgentV2CancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          AgentV2ApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) =>
          AgentV2ForbiddenException(msg, originalError: err, apiErrorCode: code),
      on404: (msg, err, code) =>
          AgentV2NotFoundException(msg, originalError: err, apiErrorCode: code),
      on409: (msg, err, code) =>
          AgentV2ConflictException(msg, originalError: err, apiErrorCode: code),
      onOtherHttp: (msg, err, code, status) => AgentV2ApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }

  Future<AgentV2Page> list({
    bool? onlyActive,
    String? executionKind,
    String? role,
    String? nameLike,
    int? limit,
    int? offset,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get(
        '/agents',
        queryParameters: {
          if (onlyActive != null) 'only_active': onlyActive,
          if (executionKind != null) 'execution_kind': executionKind,
          if (role != null) 'role': role,
          if (nameLike != null) 'name_like': nameLike,
          if (limit != null) 'limit': limit,
          if (offset != null) 'offset': offset,
        },
        cancelToken: cancelToken,
      );
      return AgentV2Page.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<AgentV2> get(String id, {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get(
        '/agents/$id',
        cancelToken: cancelToken,
      );
      return AgentV2.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<AgentV2> create({
    required String name,
    required String role,
    required String executionKind,
    String? roleDescription,
    String? systemPrompt,
    String? model,
    String? codeBackend,
    double? temperature,
    int? maxTokens,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/agents',
        cancelToken: cancelToken,
        data: {
          'name': name,
          'role': role,
          'execution_kind': executionKind,
          if (roleDescription != null) 'role_description': roleDescription,
          if (systemPrompt != null) 'system_prompt': systemPrompt,
          if (model != null && model.isNotEmpty) 'model': model,
          if (codeBackend != null && codeBackend.isNotEmpty)
            'code_backend': codeBackend,
          if (temperature != null) 'temperature': temperature,
          if (maxTokens != null) 'max_tokens': maxTokens,
          if (isActive != null) 'is_active': isActive,
        },
      );
      return AgentV2.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<AgentV2> update({
    required String id,
    String? roleDescription,
    String? systemPrompt,
    String? model,
    String? codeBackend,
    double? temperature,
    int? maxTokens,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.put(
        '/agents/$id',
        cancelToken: cancelToken,
        data: {
          if (roleDescription != null) 'role_description': roleDescription,
          if (systemPrompt != null) 'system_prompt': systemPrompt,
          if (model != null) 'model': model,
          if (codeBackend != null) 'code_backend': codeBackend,
          if (temperature != null) 'temperature': temperature,
          if (maxTokens != null) 'max_tokens': maxTokens,
          if (isActive != null) 'is_active': isActive,
        },
      );
      return AgentV2.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  /// Сохраняет/обновляет секрет (plaintext value уходит на backend, где AES-GCM
  /// шифрует и сохраняет в `agent_secrets`). Read-back невозможен.
  Future<AgentV2SecretRef> setSecret({
    required String agentId,
    required String keyName,
    required String value,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/agents/$agentId/secrets',
        cancelToken: cancelToken,
        data: {
          'key_name': keyName,
          'value': value,
        },
      );
      return AgentV2SecretRef.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> deleteSecret(String secretId, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete(
        '/agents/secrets/$secretId',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
