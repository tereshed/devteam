import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

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
    String? accessToken,
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

    // Добавляем логгер для отладки
    _dio.interceptors.add(
      LogInterceptor(
        requestBody: true,
        responseBody: true,
        logPrint: (object) => debugPrint('DIO: $object'),
      ),
    );
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
