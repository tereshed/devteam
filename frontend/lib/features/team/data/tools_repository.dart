import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/team/domain/models/tool_definition_model.dart';
import 'package:frontend/features/team/domain/tools_exceptions.dart';

/// HTTP-слой каталога инструментов (13.3.1).
class ToolsRepository {
  ToolsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          ToolsCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          ToolsApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => ToolsApiException(
        msg,
        statusCode: 403,
        originalError: err,
      ),
      on404: (msg, err, code) => ToolsApiException(
        msg,
        statusCode: 404,
        originalError: err,
      ),
      on409: null,
      onOtherHttp: (msg, err, code, status) => ToolsApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }

  /// GET /tool-definitions — активный каталог для UI.
  Future<List<ToolDefinitionModel>> fetchToolDefinitions({
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.get(
        '/tool-definitions',
        cancelToken: cancelToken,
      );
      final data = response.data;
      if (data is! List<dynamic>) {
        throw ToolsApiException('Invalid tool-definitions response');
      }
      return data
          .map((json) =>
              ToolDefinitionModel.fromJson(json as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
