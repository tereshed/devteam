import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

class MCPRegistryRepository {
  MCPRegistryRepository({required Dio dio}) : _dio = dio;
  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => AgentConfigCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) => AgentConfigApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => AgentConfigForbiddenException(msg, originalError: err, apiErrorCode: code),
      on404: (msg, err, code) => AgentConfigNotFoundException(msg, originalError: err, apiErrorCode: code),
      on409: (msg, err, code) => AgentConfigConflictException(msg, originalError: err, apiErrorCode: code),
      onOtherHttp: (msg, err, code, status) => AgentConfigApiException(msg, statusCode: status, originalError: err, apiErrorCode: code),
    );
  }

  Future<List<MCPServerRegistryModel>> list({bool? onlyActive, CancelToken? cancelToken}) async {
    try {
      final query = <String, dynamic>{};
      if (onlyActive == true) query['only_active'] = 'true';
      final resp = await _dio.get<List<dynamic>>(
        '/admin/mcp-servers',
        queryParameters: query,
        cancelToken: cancelToken,
      );
      return resp.data!.map((e) => MCPServerRegistryModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<MCPServerRegistryModel> get(String id, {CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/admin/mcp-servers/$id',
        cancelToken: cancelToken,
      );
      return MCPServerRegistryModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<MCPServerRegistryModel> create({
    required String name,
    required String transport,
    String? description,
    String? command,
    List<String>? args,
    String? url,
    Map<String, dynamic>? envTemplate,
    Map<String, dynamic>? headersTemplate,
    String? scope,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    try {
      final body = <String, dynamic>{
        'name': name,
        'transport': transport,
        if (description != null) 'description': description,
        if (command != null) 'command': command,
        if (args != null) 'args': args,
        if (url != null) 'url': url,
        if (envTemplate != null) 'env_template': envTemplate,
        if (headersTemplate != null) 'headers_template': headersTemplate,
        if (scope != null) 'scope': scope,
        if (isActive != null) 'is_active': isActive,
      };
      final resp = await _dio.post<Map<String, dynamic>>(
        '/admin/mcp-servers',
        data: body,
        cancelToken: cancelToken,
      );
      return MCPServerRegistryModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<MCPServerRegistryModel> update(
    String id, {
    required String name,
    required String transport,
    String? description,
    String? command,
    List<String>? args,
    String? url,
    Map<String, dynamic>? envTemplate,
    Map<String, dynamic>? headersTemplate,
    String? scope,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    try {
      final body = <String, dynamic>{
        'name': name,
        'transport': transport,
        if (description != null) 'description': description,
        if (command != null) 'command': command,
        if (args != null) 'args': args,
        if (url != null) 'url': url,
        if (envTemplate != null) 'env_template': envTemplate,
        if (headersTemplate != null) 'headers_template': headersTemplate,
        if (scope != null) 'scope': scope,
        if (isActive != null) 'is_active': isActive,
      };
      final resp = await _dio.put<Map<String, dynamic>>(
        '/admin/mcp-servers/$id',
        data: body,
        cancelToken: cancelToken,
      );
      return MCPServerRegistryModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> delete(String id, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete(
        '/admin/mcp-servers/$id',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
