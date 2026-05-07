import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

import 'package:frontend/core/api/dio_message_path.dart';

/// DioClient настраивает HTTP клиент для работы с API
///
/// Используется для всех запросов к backend API.
/// Включает базовую конфигурацию, interceptors для авторизации,
/// обработки ошибок и логирования.
class DioClient {
  final Dio _dio;
  final String? Function()? _getToken;

  DioClient({
    required String baseUrl,
    String? Function()? getToken,
  }) : _dio = Dio(
         BaseOptions(
           baseUrl: baseUrl,
           connectTimeout: const Duration(seconds: 30),
           receiveTimeout: const Duration(seconds: 30),
           headers: {
             'Content-Type': 'application/json',
             'Accept': 'application/json',
           },
         ),
       ),
       _getToken = getToken {
    // НЕ устанавливаем токен в headers при инициализации,
    // так как он будет добавляться динамически через interceptor

    // Добавляем interceptor для автоматического добавления токена
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          // Динамически получаем токен при каждом запросе
          final token = _getToken?.call();
          if (token != null && token.isNotEmpty) {
            // Всегда устанавливаем актуальный токен из provider
            options.headers['Authorization'] = 'Bearer $token';
          } else {
            // Если токена нет, удаляем заголовок
            options.headers.remove('Authorization');
          }
          handler.next(options);
        },
        onError: (error, handler) {
          // Обработка ошибок (401, 403, 500 и т.д.)
          handler.next(error);
        },
      ),
    );

    // Логи только в debug; тела запросов/ответов для `/conversations/.../messages` не пишем (PII).
    if (kDebugMode) {
      _dio.interceptors.add(_DebugDioLogInterceptor());
    }
  }

  /// Получить экземпляр Dio для использования в репозиториях
  Dio get dio => _dio;

  /// Обновить токен авторизации (устаревший метод, используется getToken)
  @Deprecated('Используйте getToken callback в конструкторе')
  void updateToken(String? token) {
    if (token != null) {
      _dio.options.headers['Authorization'] = 'Bearer $token';
    } else {
      _dio.options.headers.remove('Authorization');
    }
  }
}

class _DebugDioLogInterceptor extends Interceptor {
  void _log(String line) => debugPrint('DIO: $line');

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    final omitBody = isConversationMessagesApiPath(options.path) ||
        isTaskMessagesApiPath(options.path) ||
        isTaskCorrectApiPath(options.path);
    _log('--> ${options.method} ${options.uri}');
    if (omitBody) {
      _log('request body: [omitted: sensitive payload]');
    } else if (options.data != null) {
      _log('request body: ${options.data}');
    }
    handler.next(options);
  }

  @override
  void onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) {
    final omitBody =
        isConversationMessagesApiPath(response.requestOptions.path) ||
            isTaskMessagesApiPath(response.requestOptions.path) ||
            isTaskCorrectApiPath(response.requestOptions.path);
    _log('<-- ${response.statusCode} ${response.requestOptions.uri}');
    if (omitBody) {
      _log('response body: [omitted: sensitive payload]');
    } else {
      _log('response body: ${response.data}');
    }
    handler.next(response);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final ro = err.requestOptions;
    final omitBody = isConversationMessagesApiPath(ro.path) ||
        isTaskMessagesApiPath(ro.path) ||
        isTaskCorrectApiPath(ro.path);
    _log('*** ERROR ${err.response?.statusCode} ${ro.uri} (${err.type})');
    if (omitBody) {
      _log('error response body: [omitted: sensitive payload]');
    } else {
      final data = err.response?.data;
      if (data != null) {
        _log('error response body: $data');
      }
    }
    handler.next(err);
  }
}
