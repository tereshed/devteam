import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_client.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'dio_providers.g.dart';

/// Канонический [Dio] для REST API (baseUrl, auth interceptor, логирование).
///
/// Живёт в `core/api`, чтобы фичи (`auth`, `chat`, `projects`, admin) не зависели
/// от `part`-артефактов чужих модулей.
@Riverpod(keepAlive: true)
Dio dioClient(Ref ref) {
  // TODO: Получить baseUrl из конфигурации
  const baseUrl = 'http://127.0.0.1:8080/api/v1';

  String? getToken() => ref.read(accessTokenProvider);

  final client = DioClient(
    baseUrl: baseUrl,
    getToken: getToken,
  );

  return client.dio;
}
