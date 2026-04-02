import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:frontend/core/api/dio_client.dart';
import 'package:frontend/core/storage/token_provider.dart';

part 'auth_providers.g.dart';

/// Provider для Dio клиента
///
/// Настраивает базовый HTTP клиент для работы с API.
/// Автоматически обновляется при изменении токена через interceptor.
@Riverpod(keepAlive: true)
Dio dioClient(Ref ref) {
  // TODO: Получить baseUrl из конфигурации
  const baseUrl = 'http://127.0.0.1:8080/api/v1';

  // Получаем начальный токен из provider
  final initialToken = ref.read(accessTokenProvider);

  // Создаем функцию для динамического получения токена при каждом запросе
  // Используем замыкание на ref для доступа к актуальному значению provider
  String? getToken() {
    // Получаем актуальное значение токена из provider при каждом запросе
    final token = ref.read(accessTokenProvider);
    return token;
  }

  final dioClient = DioClient(
    baseUrl: baseUrl,
    accessToken: initialToken,
    getToken: getToken,
  );

  return dioClient.dio;
}

/// Provider для AuthRepository
///
/// Предоставляет экземпляр AuthRepository с настроенным Dio клиентом.
@Riverpod(keepAlive: true)
AuthRepository authRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return AuthRepository(dio: dio);
}
