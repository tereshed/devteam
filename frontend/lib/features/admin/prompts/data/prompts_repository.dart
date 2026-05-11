import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_exceptions.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_model.dart';

/// HTTP-слой промптов (админка + диалог команды 13.3).
class PromptsRepository {
  PromptsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Map<String, dynamic> _jsonMap(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw PromptApiException(
          msg,
          statusCode: code,
        ),
      );

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          PromptCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err) =>
          PromptApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => PromptForbiddenException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on404: (msg, err, code) => PromptNotFoundException(
        msg,
        originalError: err,
        apiErrorCode: code,
      ),
      on409: null,
      onOtherHttp: (msg, err, code, status) => PromptApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }

  Future<List<Prompt>> getPrompts({CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get(
        '/prompts',
        cancelToken: cancelToken,
      );
      final data = response.data;
      if (data is! List<dynamic>) {
        throw PromptApiException('Invalid prompts response');
      }
      return data
          .map((json) => Prompt.fromJson(json as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<Prompt> getPrompt(String id, {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get(
        '/prompts/$id',
        cancelToken: cancelToken,
      );
      return Prompt.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<Prompt> createPrompt({
    required String name,
    required String description,
    required String template,
    Map<String, dynamic>? jsonSchema,
    bool isActive = true,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/prompts',
        cancelToken: cancelToken,
        data: {
          'name': name,
          'description': description,
          'template': template,
          'json_schema': jsonSchema,
          'is_active': isActive,
        },
      );
      return Prompt.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<Prompt> updatePrompt({
    required String id,
    String? description,
    String? template,
    Map<String, dynamic>? jsonSchema,
    bool? isActive,
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.put(
        '/prompts/$id',
        cancelToken: cancelToken,
        data: {
          if (description != null) 'description': description,
          if (template != null) 'template': template,
          if (jsonSchema != null) 'json_schema': jsonSchema,
          if (isActive != null) 'is_active': isActive,
        },
      );
      return Prompt.fromJson(_jsonMap(response));
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> deletePrompt(String id, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete(
        '/prompts/$id',
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
