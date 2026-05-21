import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/team/domain/agent_config_exceptions.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

class RolePromptsRepository {
  RolePromptsRepository({required Dio dio}) : _dio = dio;
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

  Future<List<AgentRolePromptModel>> list({CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.get<List<dynamic>>(
        '/admin/agent-role-prompts',
        cancelToken: cancelToken,
      );
      return resp.data!.map((e) => AgentRolePromptModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<AgentRolePromptModel> getByRole(String role, {CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/admin/agent-role-prompts/$role',
        cancelToken: cancelToken,
      );
      return AgentRolePromptModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<AgentRolePromptModel> update(String role, {required String content, String? description, CancelToken? cancelToken}) async {
    try {
      final body = <String, dynamic>{
        'content': content,
        if (description != null) 'description': description,
      };
      final resp = await _dio.put<Map<String, dynamic>>(
        '/admin/agent-role-prompts/$role',
        data: body,
        cancelToken: cancelToken,
      );
      return AgentRolePromptModel.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
