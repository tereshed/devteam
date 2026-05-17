import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_client.dart';
import 'package:frontend/core/api/refresh_auth_interceptor.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:frontend/core/storage/token_storage.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'dio_providers.g.dart';

/// Канонический [Dio] для REST API (baseUrl, auth interceptor, логирование).
///
/// Живёт в `core/api`, чтобы фичи (`auth`, `chat`, `projects`, admin) не зависели
/// от `part`-артефактов чужих модулей.
@Riverpod(keepAlive: true)
Dio dioClient(Ref ref) {
  // TODO: Получить baseUrl из конфигурации.
  // Используем localhost (не 127.0.0.1): GitHub OAuth App для dev зарегистрирован
  // с redirect_uri http://localhost:8080/... — браузер и GitHub различают хосты,
  // 127.0.0.1 даст "redirect_uri is not associated" (см. docs/tasks/ui_refactoring/oauth-setup-guide.md §1).
  const baseUrl = 'http://localhost:8080/api/v1';

  String? getToken() => ref.read(accessTokenProvider);

  final client = DioClient(
    baseUrl: baseUrl,
    getToken: getToken,
  );

  // 401 → silent refresh + retry. Без него короткоживущий JWT (15 мин по
  // умолчанию) делает любой долгоживущий экран мёртвым после истечения.
  client.dio.interceptors.add(
    RefreshAuthInterceptor(
      dio: client.dio,
      getRefreshToken: TokenStorage.getRefreshToken,
      onRefreshed: (access, refresh) async {
        await ref.read(accessTokenProvider.notifier).setToken(access);
        await TokenStorage.saveRefreshToken(refresh);
      },
      onRefreshFailed: () async {
        await ref.read(accessTokenProvider.notifier).clear();
      },
    ),
  );

  return client.dio;
}
