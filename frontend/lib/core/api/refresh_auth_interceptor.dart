import 'dart:async';

import 'package:dio/dio.dart';

/// Перехватчик 401: пытается обновить access-token через `POST /auth/refresh`,
/// затем повторяет оригинальный запрос ровно один раз.
///
/// Single-flight: все параллельные 401-запросы делят один refresh-вызов
/// (если 10 виджетов одновременно прилетели в 401, мы делаем 1 refresh,
/// а не 10). Refresh-эндпоинт и сами /auth/* пути в обход — иначе бесконечная
/// рекурсия.
///
/// На неуспешный refresh вызывается [onRefreshFailed] (UI должен трактовать
/// как логаут) и оригинальная ошибка пробрасывается дальше — слой выше получит
/// `UnauthorizedException`.
class RefreshAuthInterceptor extends Interceptor {
  RefreshAuthInterceptor({
    required Dio dio,
    required Future<String?> Function() getRefreshToken,
    required Future<void> Function(String accessToken, String refreshToken)
        onRefreshed,
    required Future<void> Function() onRefreshFailed,
  })  : _dio = dio,
        _getRefreshToken = getRefreshToken,
        _onRefreshed = onRefreshed,
        _onRefreshFailed = onRefreshFailed;

  final Dio _dio;
  final Future<String?> Function() _getRefreshToken;
  final Future<void> Function(String accessToken, String refreshToken)
      _onRefreshed;
  final Future<void> Function() _onRefreshFailed;

  // Маркер в RequestOptions.extra — чтобы один и тот же запрос не уходил
  // в refresh-loop, если повторная попытка тоже вернёт 401.
  static const String _retriedKey = '__auth_refresh_retried__';

  Future<String?>? _inFlightRefresh;

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final status = err.response?.statusCode;
    final path = err.requestOptions.path;
    final alreadyRetried = err.requestOptions.extra[_retriedKey] == true;
    final isAuthEndpoint = path.contains('/auth/refresh') ||
        path.contains('/auth/login') ||
        path.contains('/auth/register');

    if (status != 401 || alreadyRetried || isAuthEndpoint) {
      handler.next(err);
      return;
    }

    final newToken = await _refresh();
    if (newToken == null) {
      handler.next(err);
      return;
    }

    final retryOptions = err.requestOptions
      ..extra[_retriedKey] = true
      ..headers['Authorization'] = 'Bearer $newToken';

    try {
      final response = await _dio.fetch<dynamic>(retryOptions);
      handler.resolve(response);
    } on DioException catch (e) {
      handler.next(e);
    }
  }

  Future<String?> _refresh() async {
    final inFlight = _inFlightRefresh;
    if (inFlight != null) {
      return inFlight;
    }
    final completer = Completer<String?>();
    _inFlightRefresh = completer.future;
    try {
      final refreshToken = await _getRefreshToken();
      if (refreshToken == null || refreshToken.isEmpty) {
        await _onRefreshFailed();
        completer.complete(null);
        return null;
      }
      // Помечаем сам refresh-запрос как retried, чтобы наш же interceptor его
      // не перехватил (path-check уже исключает /auth/refresh, но extra-маркер —
      // вторая страховка на случай переименования эндпоинта).
      final response = await _dio.post<Map<String, dynamic>>(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
        options: Options(extra: {_retriedKey: true}),
      );
      final data = response.data;
      final newAccess = data?['access_token'] as String?;
      final newRefresh = data?['refresh_token'] as String?;
      if (newAccess == null || newRefresh == null || newAccess.isEmpty) {
        await _onRefreshFailed();
        completer.complete(null);
        return null;
      }
      await _onRefreshed(newAccess, newRefresh);
      completer.complete(newAccess);
      return newAccess;
    } catch (_) {
      await _onRefreshFailed();
      completer.complete(null);
      return null;
    } finally {
      _inFlightRefresh = null;
    }
  }
}
