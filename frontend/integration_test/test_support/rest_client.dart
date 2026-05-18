import 'dart:convert';
import 'dart:io';

import 'api_base.dart';

/// Унифицированная ошибка REST-вызова для интеграционных тестов.
///
/// `toString()` сразу включает statusCode и тело ответа — это критично для
/// CI: при падении теста в логах видно конкретный ответ бэка, а не только
/// «status=500». Размер тела не обрезаем: тестовые ответы обычно ≤2KB.
class RestRequestException implements Exception {
  RestRequestException({
    required this.method,
    required this.path,
    required this.status,
    required this.body,
  });

  final String method;
  final String path;
  final int status;
  final String body;

  @override
  String toString() => 'REST $method $path failed: status=$status body=$body';
}

/// Тонкая HTTP-обёртка для интеграционных тестов.
///
/// Цели:
///   1. **DRY**: один способ слать REST в backend — никаких локальных
///      `HttpClient()` в seed-хелперах и тест-файлах.
///   2. **Единая обработка ошибок**: любой не-2xx → [RestRequestException]
///      с телом ответа в `toString()` (упрощает дебаг CI-падений).
///   3. **Гарантия отсутствия утечек сокетов**: `transform(utf8.decoder)`
///      внутри `_request` вычитывает поток до конца ДО любого `throw`, а
///      `client.close(force: true)` в `finally` гарантированно освобождает
///      file descriptors даже на упавшем сетевом стеке.
///
/// Не предназначен для дёргания LLM-эндпоинтов из Flutter-тестов: backend
/// для Phase 3 поднят с FakeLLM-redirect + dummy-ключами, но любой вызов
/// `/llm/...` всё равно попадёт в `llm_logs` и сломает cost-leak guard
/// (см. `backend/test/featuresmoke/frontend_e2e_test.go`).
class TestRestClient {
  // Конструктор закрыт: вся API статическая. Если когда-нибудь понадобится
  // configurable instance (например, для real-mode с своим baseUrl), можно
  // безболезненно превратить в обычный класс — все вызывающие места
  // переписать одной заменой `TestRestClient.foo(` → `client.foo(`.
  TestRestClient._();

  /// 2xx-коды, которые считаем успехом. 204 нужен для DELETE'ов
  /// (когда мы их добавим), 202 — для async-эндпоинтов вроде reindex.
  static const _okStatuses = <int>{200, 201, 202, 204};

  /// Низкоуровневый запрос. Возвращает декодированный JSON-объект (`{}` для
  /// пустого тела). Бросает [RestRequestException] на любом не-2xx.
  ///
  /// `path` — путь относительно [kApiV1]; начинается с «/».
  /// `body` — любой JSON-сериализуемый объект (Map/List/etc).
  static Future<Map<String, dynamic>> _request(
    String method,
    String path, {
    String? token,
    Object? body,
  }) async {
    final uri = Uri.parse('$kApiV1$path');
    final client = HttpClient()..connectionTimeout = const Duration(seconds: 5);
    try {
      final req = await _openRequest(client, method, uri);
      if (body != null) {
        req.headers.set('Content-Type', 'application/json');
        req.write(jsonEncode(body));
      }
      if (token != null && token.isNotEmpty) {
        req.headers.set('Authorization', 'Bearer $token');
      }
      final resp = await req.close();
      // transform(utf8.decoder).join() гарантированно дренит поток, даже если
      // дальше мы бросаем исключение — отдельный resp.drain() не нужен.
      final raw = await resp.transform(utf8.decoder).join();
      if (!_okStatuses.contains(resp.statusCode)) {
        throw RestRequestException(
          method: method,
          path: path,
          status: resp.statusCode,
          body: raw,
        );
      }
      if (raw.isEmpty) {
        return const <String, dynamic>{};
      }
      return jsonDecode(raw) as Map<String, dynamic>;
    } finally {
      // force:true — на случай зависшего/упавшего соединения. Без него
      // teardown integration-test'а блокировался бы до httpClient.idleTimeout.
      client.close(force: true);
    }
  }

  static Future<HttpClientRequest> _openRequest(
    HttpClient client,
    String method,
    Uri uri,
  ) {
    switch (method) {
      case 'GET':
        return client.getUrl(uri);
      case 'POST':
        return client.postUrl(uri);
      case 'PUT':
        return client.putUrl(uri);
      case 'PATCH':
        return client.patchUrl(uri);
      case 'DELETE':
        return client.deleteUrl(uri);
      default:
        throw ArgumentError('TestRestClient: unsupported method "$method"');
    }
  }

  static Future<Map<String, dynamic>> get(String path, {String? token}) =>
      _request('GET', path, token: token);

  static Future<Map<String, dynamic>> post(
    String path, {
    String? token,
    Object? body,
  }) => _request('POST', path, token: token, body: body);

  static Future<Map<String, dynamic>> put(
    String path, {
    String? token,
    Object? body,
  }) => _request('PUT', path, token: token, body: body);

  static Future<Map<String, dynamic>> patch(
    String path, {
    String? token,
    Object? body,
  }) => _request('PATCH', path, token: token, body: body);

  /// Health-check на корневой `/health` (он живёт ВНЕ /api/v1).
  /// Возвращает `true` при 200, иначе `false` (без исключений — используется
  /// `ensureBackendOrSkip`, которому нужен soft-сигнал, а не throw).
  static Future<bool> healthCheck() async {
    final client = HttpClient()..connectionTimeout = const Duration(seconds: 2);
    try {
      final req = await client.getUrl(Uri.parse('$kApiBase/health'));
      final resp = await req.close().timeout(const Duration(seconds: 3));
      await resp.drain<void>();
      return resp.statusCode == 200;
    } on Exception {
      return false;
    } finally {
      client.close(force: true);
    }
  }
}
