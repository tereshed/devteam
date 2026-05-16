import 'package:dio/dio.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';

/// HTTP-слой экрана Git Integrations (UI Refactoring §5 — Этап 3b).
///
/// Маршруты — зеркало `server.go` (Этап 3a):
///   * `POST   /integrations/{provider}/auth/init`
///   * `POST   /integrations/{provider}/auth/callback`
///   * `GET    /integrations/{provider}/auth/status`
///   * `DELETE /integrations/{provider}/auth/revoke`
///
/// Контракт ошибок — узкий [GitIntegrationsException] с `errorCode`+`statusCode`,
/// чтобы UI различал §4a.5 кейсы (`user_cancelled`/`invalid_state`/`provider_unreachable`/
/// `invalid_host`/`oauth_not_configured`) без иерархии подклассов.
class GitIntegrationsRepository {
  GitIntegrationsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  /// Старт OAuth-flow для GitHub или gitlab.com (Shared).
  ///
  /// Для self-hosted GitLab используйте именованную форму с [host]/[byoClientId]/
  /// [byoClientSecret] — бэк опредeлит вариант по непустому `host`.
  Future<GitOAuthInitResult> init(
    GitIntegrationProvider provider, {
    required String redirectUri,
    String? host,
    String? byoClientId,
    String? byoClientSecret,
    CancelToken? cancelToken,
  }) async {
    final body = <String, dynamic>{'redirect_uri': redirectUri};
    if (host != null && host.isNotEmpty) {
      body['host'] = host;
    }
    if (byoClientId != null && byoClientId.isNotEmpty) {
      body['byo_client_id'] = byoClientId;
    }
    if (byoClientSecret != null && byoClientSecret.isNotEmpty) {
      body['byo_client_secret'] = byoClientSecret;
    }
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/integrations/${provider.jsonValue}/auth/init',
        data: body,
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return GitOAuthInitResult(
        authorizeUrl: (data['authorize_url'] as String?) ?? '',
        state: (data['state'] as String?) ?? '',
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Завершение OAuth (передача `code` + `state` от провайдера).
  ///
  /// При `?error=...` от провайдера фронт передаёт его через [error] — бэк отдаст
  /// `410 user_cancelled` (§4a.5).
  Future<GitProviderConnection> completeCallback(
    GitIntegrationProvider provider, {
    required String code,
    required String state,
    String? error,
    CancelToken? cancelToken,
  }) async {
    final body = <String, dynamic>{'code': code, 'state': state};
    if (error != null && error.isNotEmpty) {
      body['error'] = error;
    }
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/integrations/${provider.jsonValue}/auth/callback',
        data: body,
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      final status = data['status'];
      if (status is! Map<String, dynamic>) {
        throw const GitIntegrationsException(
          message: 'callback: missing status payload',
        );
      }
      return _parseStatus(provider, status);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Получить статус подключения провайдера.
  Future<GitProviderConnection> fetchStatus(
    GitIntegrationProvider provider, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/integrations/${provider.jsonValue}/auth/status',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return _parseStatus(provider, data);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Отзыв подключения. Возвращает `remoteRevokeFailed` — `true`, если провайдер
  /// не отозвал токен (см. §4a.1 — отзыв обязателен, fail-soft в БД).
  Future<bool> revoke(
    GitIntegrationProvider provider, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.delete<Map<String, dynamic>>(
        '/integrations/${provider.jsonValue}/auth/revoke',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return data['remote_revoke_failed'] == true;
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  // --- helpers ------------------------------------------------------------

  static GitProviderConnection _parseStatus(
    GitIntegrationProvider provider,
    Map<String, dynamic> data,
  ) {
    final connected = data['connected'] == true;
    return GitProviderConnection(
      provider: provider,
      status: connected
          ? GitProviderConnectionStatus.connected
          : GitProviderConnectionStatus.disconnected,
      host: _string(data['host']),
      accountLogin: _string(data['account_login']),
      scopes: _string(data['scopes']),
      connectedAt: _parseTs(data['connected_at']),
      expiresAt: _parseTs(data['expires_at']),
    );
  }

  static String? _string(Object? raw) {
    if (raw is String && raw.isNotEmpty) {
      return raw;
    }
    return null;
  }

  static DateTime? _parseTs(Object? raw) {
    if (raw is! String || raw.isEmpty) {
      return null;
    }
    return DateTime.tryParse(raw)?.toUtc();
  }

  Exception _mapDioError(DioException error) {
    final status = error.response?.statusCode;
    final body = error.response?.data;
    String? code;
    var message = error.message ?? 'network error';
    if (body is Map<String, dynamic>) {
      final c = body['error_code'];
      if (c is String && c.isNotEmpty) {
        code = c;
      }
      final m = body['message'];
      if (m is String && m.isNotEmpty) {
        message = m;
      }
    }
    return GitIntegrationsException(
      message: message,
      statusCode: status,
      errorCode: code,
    );
  }
}

/// Ответ `POST /integrations/{provider}/auth/init`.
class GitOAuthInitResult {
  const GitOAuthInitResult({required this.authorizeUrl, required this.state});

  final String authorizeUrl;
  final String state;
}

/// Узкое исключение HTTP-слоя git-интеграций. UI различает кейсы через
/// [errorCode] (`user_cancelled`/`invalid_state`/`provider_unreachable`/
/// `invalid_host`/`oauth_not_configured`) и [statusCode], а не по типу.
class GitIntegrationsException implements Exception {
  const GitIntegrationsException({
    required this.message,
    this.statusCode,
    this.errorCode,
  });

  final String message;
  final int? statusCode;
  final String? errorCode;

  @override
  String toString() =>
      'GitIntegrationsException(status=$statusCode, code=$errorCode, message=$message)';
}
