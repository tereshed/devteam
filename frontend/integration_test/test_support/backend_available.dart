import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

import 'api_base.dart';
import 'rest_client.dart';

/// Возвращает true, если backend по [kApiBase] отвечает 200 на `/health`.
///
/// Тонкая обёртка над [TestRestClient.healthCheck] — оставлена как явная
/// точка для тестов, которые хотят сами решать skip-vs-fail. В CI этим
/// занимается [ensureBackendOrSkip].
Future<bool> backendAvailable() => TestRestClient.healthCheck();

/// Хелпер с единым сообщением: вызывается в начале `testWidgets`.
///
/// Возвращает true, если тест следует продолжить; false — если бэка нет
/// и **пропуск разрешён** (локальная разработка). В CI (`FEATURESMOKE_REQUIRE_BACKEND=1`)
/// падаем явным `fail`.
Future<bool> ensureBackendOrSkip() async {
  if (await backendAvailable()) {
    return true;
  }
  const msg =
      'backend at $kApiBase is not reachable; start the stack '
      'via `make test-features-frontend` (или `docker compose up -d` + бинарь).';
  // Контракт CI: FEATURESMOKE_REQUIRE_BACKEND=1 → backend обязателен,
  // иначе джоба падает с понятной ошибкой вместо «зелёного skip».
  if (Platform.environment['FEATURESMOKE_REQUIRE_BACKEND'] == '1') {
    fail(msg);
  }
  markTestSkipped(msg);
  return false;
}
