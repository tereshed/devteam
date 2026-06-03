import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_api_error.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/webhooks/domain/models/webhook_model.dart';

class WebhookApiException extends ApiException {
  final int? statusCode;
  WebhookApiException(super.message, {this.statusCode, super.originalError});
}

class WebhookRepository {
  final Dio _dio;

  WebhookRepository({required Dio dio}) : _dio = dio;

  Map<String, dynamic> _jsonBody(Response<dynamic> response) =>
      requireResponseJsonMap(
        response,
        onInvalid: (msg, code) => throw WebhookApiException(
          msg,
          statusCode: code,
        ),
      );

  List<dynamic> _jsonList(Response<dynamic> response) =>
      requireResponseJsonList(
        response,
        onInvalid: (msg, code) => throw WebhookApiException(
          msg,
          statusCode: code,
        ),
      );

  Future<List<WebhookModel>> listWebhooks({CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get('/webhooks', cancelToken: cancelToken);
      final list = _jsonList(response);
      return list.map((e) => WebhookModel.fromJson(e as Map<String, dynamic>)).toList();
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<WebhookModel> getWebhook(String id, {CancelToken? cancelToken}) async {
    try {
      final response = await _dio.get('/webhooks/$id', cancelToken: cancelToken);
      return WebhookModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<WebhookModel> createWebhook(
    CreateWebhookRequest request, {
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.post(
        '/webhooks',
        data: request.toJson(),
        cancelToken: cancelToken,
      );
      return WebhookModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<WebhookModel> updateWebhook(
    String id,
    UpdateWebhookRequest request, {
    CancelToken? cancelToken,
  }) async {
    try {
      final response = await _dio.put(
        '/webhooks/$id',
        data: request.toJson(),
        cancelToken: cancelToken,
      );
      return WebhookModel.fromJson(_jsonBody(response));
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Future<void> deleteWebhook(String id, {CancelToken? cancelToken}) async {
    try {
      await _dio.delete('/webhooks/$id', cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _handleDioError(e);
    }
  }

  Exception _handleDioError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) => WebhookApiException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) => WebhookApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) => WebhookApiException(msg, originalError: err, statusCode: 403),
      on404: (msg, err, code) => WebhookApiException(msg, originalError: err, statusCode: 404),
      onOtherHttp: (msg, err, code, status) => WebhookApiException(
        msg,
        statusCode: status,
        originalError: err,
      ),
    );
  }
}
