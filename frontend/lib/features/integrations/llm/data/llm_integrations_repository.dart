import 'package:dio/dio.dart';
import 'package:frontend/features/integrations/llm/domain/antigravity_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/claude_code_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

/// Aggregated HTTP-слой экрана LLM Integrations (UI Refactoring §5 — Этап 2).
///
/// Объединяет три эндпоинта, чтобы экран не зависел от их физического разделения:
///   * `GET /me/llm-credentials` — маски ключей API-key провайдеров.
///   * `GET /claude-code/auth/status` — статус OAuth-подписки.
///   * `POST/DELETE /me/llm-credentials` + `/claude-code/auth/*` — мутации.
///
/// Контракт сетевых ошибок намеренно простой: бросаем [LlmIntegrationsException]
/// со статус-кодом, чтобы UI мог различить cancel/network/forbidden по полю
/// `errorCode`/`statusCode` без специальных подклассов.
class LlmIntegrationsRepository {
  LlmIntegrationsRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;

  /// Получить список API-key провайдеров (маска + provider id) для текущего юзера.
  ///
  /// Список **только** API-key каналов — Claude Code OAuth получается отдельно через
  /// [fetchClaudeCodeStatus], потому что у него своя таблица и свой контракт.
  Future<List<LlmProviderConnection>> fetchApiKeyConnections({
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/me/llm-credentials',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return _parseCredentialsResponse(data);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Получить статус OAuth-подписки Claude Code.
  Future<ClaudeCodeIntegrationStatus> fetchClaudeCodeStatus({
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/claude-code/auth/status',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return ClaudeCodeIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Получить статус OAuth-подписки Antigravity.
  Future<AntigravityIntegrationStatus> fetchAntigravityStatus({
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/antigravity/auth/status',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return AntigravityIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Сохранить API-key для одного из API-key провайдеров.
  /// Бросает [LlmIntegrationsException], если provider — `claude_code_oauth`
  /// (для OAuth-flow вызывайте [initClaudeCodeOAuth]/[completeClaudeCodeOAuth]).
  Future<void> setApiKey({
    required LlmIntegrationProvider provider,
    required String apiKey,
    CancelToken? cancelToken,
  }) async {
    final field = _patchFieldForProvider(provider);
    try {
      await _dio.patch<Map<String, dynamic>>(
        '/me/llm-credentials',
        data: <String, dynamic>{field: apiKey},
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Удалить сохранённый API-key конкретного провайдера.
  Future<void> clearApiKey({
    required LlmIntegrationProvider provider,
    CancelToken? cancelToken,
  }) async {
    final clearField = _clearFieldForProvider(provider);
    try {
      await _dio.patch<Map<String, dynamic>>(
        '/me/llm-credentials',
        data: <String, dynamic>{clearField: true},
        cancelToken: cancelToken,
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Старт OAuth device-flow Claude Code. Возвращает данные для отображения юзеру.
  Future<ClaudeCodeOAuthInit> initClaudeCodeOAuth({
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/claude-code/auth/init',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return ClaudeCodeOAuthInit(
        deviceCode: (data['device_code'] as String?) ?? '',
        userCode: (data['user_code'] as String?) ?? '',
        verificationUri: (data['verification_uri'] as String?) ?? '',
        verificationUriComplete:
            (data['verification_uri_complete'] as String?) ?? '',
        intervalSeconds: (data['interval_seconds'] is int)
            ? data['interval_seconds'] as int
            : 5,
        expiresInSeconds: (data['expires_in_seconds'] is int)
            ? data['expires_in_seconds'] as int
            : 900,
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Один poll callback'а: проверка статуса OAuth device-flow.
  ///
  /// Возвращает [ClaudeCodeIntegrationStatus.connected]=true при успехе.
  /// `authorization_pending` приходит как 202 → бросается
  /// [LlmIntegrationsException] с `errorCode = "authorization_pending"`.
  Future<ClaudeCodeIntegrationStatus> completeClaudeCodeOAuth({
    required String deviceCode,
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/claude-code/auth/callback',
        data: <String, dynamic>{'device_code': deviceCode},
        cancelToken: cancelToken,
      );
      if (resp.statusCode == 202) {
        throw const LlmIntegrationsException(
          message: 'authorization_pending',
          errorCode: 'authorization_pending',
          statusCode: 202,
        );
      }
      final data = resp.data ?? <String, dynamic>{};
      return ClaudeCodeIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  Future<void> revokeClaudeCodeOAuth({CancelToken? cancelToken}) async {
    try {
      await _dio.delete<dynamic>('/claude-code/auth', cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Сохраняет уже полученный пользователем OAuth-токен Claude Code (например,
  /// результат `claude setup-token`). Альтернатива device-flow для случаев,
  /// когда `CLAUDE_CODE_OAUTH_CLIENT_ID` на бэке не задан.
  Future<ClaudeCodeIntegrationStatus> saveClaudeCodeManualToken({
    required String accessToken,
    String? refreshToken,
    DateTime? expiresAt,
    String? scopes,
    CancelToken? cancelToken,
  }) async {
    try {
      final body = <String, dynamic>{'access_token': accessToken};
      if (refreshToken != null && refreshToken.isNotEmpty) {
        body['refresh_token'] = refreshToken;
      }
      if (expiresAt != null) {
        body['expires_at'] = expiresAt.toUtc().toIso8601String();
      }
      if (scopes != null && scopes.isNotEmpty) {
        body['scopes'] = scopes;
      }
      final resp = await _dio.put<Map<String, dynamic>>(
        '/claude-code/auth/manual-token',
        data: body,
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return ClaudeCodeIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Старт OAuth device-flow Antigravity. Возвращает данные для отображения юзеру.
  Future<AntigravityOAuthInit> initAntigravityOAuth({
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/antigravity/auth/init',
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return AntigravityOAuthInit(
        deviceCode: (data['device_code'] as String?) ?? '',
        userCode: (data['user_code'] as String?) ?? '',
        verificationUri: (data['verification_uri'] as String?) ?? '',
        verificationUriComplete:
            (data['verification_uri_complete'] as String?) ?? '',
        intervalSeconds: (data['interval_seconds'] is int)
            ? data['interval_seconds'] as int
            : 5,
        expiresInSeconds: (data['expires_in_seconds'] is int)
            ? data['expires_in_seconds'] as int
            : 900,
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Один poll callback'а: проверка статуса OAuth device-flow для Antigravity.
  ///
  /// Возвращает [AntigravityIntegrationStatus.connected]=true при успехе.
  /// `authorization_pending` приходит как 202 → бросается
  /// [LlmIntegrationsException] с `errorCode = "authorization_pending"`.
  Future<AntigravityIntegrationStatus> completeAntigravityOAuth({
    required String deviceCode,
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/antigravity/auth/callback',
        data: <String, dynamic>{'device_code': deviceCode},
        cancelToken: cancelToken,
      );
      if (resp.statusCode == 202) {
        throw const LlmIntegrationsException(
          message: 'authorization_pending',
          errorCode: 'authorization_pending',
          statusCode: 202,
        );
      }
      final data = resp.data ?? <String, dynamic>{};
      return AntigravityIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  Future<void> revokeAntigravityOAuth({CancelToken? cancelToken}) async {
    try {
      await _dio.delete<dynamic>('/antigravity/auth', cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Сохраняет уже полученный пользователем OAuth-токен Antigravity.
  /// Альтернатива device-flow.
  Future<AntigravityIntegrationStatus> saveAntigravityManualToken({
    required String accessToken,
    String? refreshToken,
    DateTime? expiresAt,
    String? scopes,
    CancelToken? cancelToken,
  }) async {
    try {
      final body = <String, dynamic>{'access_token': accessToken};
      if (refreshToken != null && refreshToken.isNotEmpty) {
        body['refresh_token'] = refreshToken;
      }
      if (expiresAt != null) {
        body['expires_at'] = expiresAt.toUtc().toIso8601String();
      }
      if (scopes != null && scopes.isNotEmpty) {
        body['scopes'] = scopes;
      }
      final resp = await _dio.put<Map<String, dynamic>>(
        '/antigravity/auth/manual-token',
        data: body,
        cancelToken: cancelToken,
      );
      final data = resp.data ?? <String, dynamic>{};
      return AntigravityIntegrationStatus(
        connected: data['connected'] == true,
        expiresAt: _parseTs(data['expires_at']),
        lastRefreshedAt: _parseTs(data['last_refreshed_at']),
      );
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  /// Получить список доступных моделей для провайдера.
  Future<List<String>> fetchAvailableModels(
    String providerKind, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/llm/models',
        queryParameters: <String, dynamic>{'provider': providerKind},
        cancelToken: cancelToken,
      );
      final dynamic rawData = resp.data;
      if (rawData is List) {
        return rawData.map((dynamic e) => e.toString()).toList();
      }
      return <String>[];
    } on DioException catch (e) {
      throw _mapDioError(e);
    }
  }

  // --- helpers ------------------------------------------------------------

  static List<LlmProviderConnection> _parseCredentialsResponse(
    Map<String, dynamic> data,
  ) {
    final out = <LlmProviderConnection>[];
    const mapping = <String, LlmIntegrationProvider>{
      'openai': LlmIntegrationProvider.openai,
      'anthropic': LlmIntegrationProvider.anthropic,
      'gemini': LlmIntegrationProvider.gemini,
      'deepseek': LlmIntegrationProvider.deepseek,
      'qwen': LlmIntegrationProvider.qwen,
      'openrouter': LlmIntegrationProvider.openrouter,
      'zhipu': LlmIntegrationProvider.zhipu,
      'antigravity': LlmIntegrationProvider.antigravity,
      'hermes': LlmIntegrationProvider.hermes,
    };
    for (final entry in mapping.entries) {
      final raw = data[entry.key];
      if (raw is! Map<String, dynamic>) {
        continue;
      }
      final masked = raw['masked_preview'];
      if (masked is String && masked.isNotEmpty) {
        out.add(
          LlmProviderConnection(
            provider: entry.value,
            status: LlmProviderConnectionStatus.connected,
            maskedPreview: masked,
          ),
        );
      } else {
        out.add(
          LlmProviderConnection(
            provider: entry.value,
            status: LlmProviderConnectionStatus.disconnected,
          ),
        );
      }
    }
    return out;
  }

  static DateTime? _parseTs(Object? raw) {
    if (raw is! String || raw.isEmpty) {
      return null;
    }
    return DateTime.tryParse(raw)?.toUtc();
  }

  static String _patchFieldForProvider(LlmIntegrationProvider p) {
    switch (p) {
      case LlmIntegrationProvider.openai:
        return 'openai_api_key';
      case LlmIntegrationProvider.anthropic:
        return 'anthropic_api_key';
      case LlmIntegrationProvider.gemini:
        return 'gemini_api_key';
      case LlmIntegrationProvider.deepseek:
        return 'deepseek_api_key';
      case LlmIntegrationProvider.qwen:
        return 'qwen_api_key';
      case LlmIntegrationProvider.openrouter:
        return 'openrouter_api_key';
      case LlmIntegrationProvider.zhipu:
        return 'zhipu_api_key';
      case LlmIntegrationProvider.antigravity:
        return 'antigravity_api_key';
      case LlmIntegrationProvider.hermes:
        return 'hermes_api_key';
      case LlmIntegrationProvider.claudeCodeOAuth:
        throw ArgumentError.value(
          p,
          'provider',
          'API-key flow не поддерживается; используйте OAuth/admin-CRUD',
        );
      case LlmIntegrationProvider.antigravityOAuth:
        throw ArgumentError.value(
          p,
          'provider',
          'API-key flow не поддерживается; используйте OAuth/admin-CRUD',
        );
    }
  }

  static String _clearFieldForProvider(LlmIntegrationProvider p) {
    switch (p) {
      case LlmIntegrationProvider.openai:
        return 'clear_openai_key';
      case LlmIntegrationProvider.anthropic:
        return 'clear_anthropic_key';
      case LlmIntegrationProvider.gemini:
        return 'clear_gemini_key';
      case LlmIntegrationProvider.deepseek:
        return 'clear_deepseek_key';
      case LlmIntegrationProvider.qwen:
        return 'clear_qwen_key';
      case LlmIntegrationProvider.openrouter:
        return 'clear_openrouter_key';
      case LlmIntegrationProvider.zhipu:
        return 'clear_zhipu_key';
      case LlmIntegrationProvider.antigravity:
        return 'clear_antigravity_key';
      case LlmIntegrationProvider.hermes:
        return 'clear_hermes_key';
      case LlmIntegrationProvider.claudeCodeOAuth:
        throw ArgumentError.value(
          p,
          'provider',
          'API-key flow не поддерживается',
        );
      case LlmIntegrationProvider.antigravityOAuth:
        throw ArgumentError.value(
          p,
          'provider',
          'API-key flow не поддерживается',
        );
    }
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
    return LlmIntegrationsException(
      message: message,
      statusCode: status,
      errorCode: code,
    );
  }
}

/// Данные device-flow, возвращаемые `POST /claude-code/auth/init`.
class ClaudeCodeOAuthInit {
  const ClaudeCodeOAuthInit({
    required this.deviceCode,
    required this.userCode,
    required this.verificationUri,
    required this.verificationUriComplete,
    required this.intervalSeconds,
    required this.expiresInSeconds,
  });

  final String deviceCode;
  final String userCode;
  final String verificationUri;
  final String verificationUriComplete;
  final int intervalSeconds;
  final int expiresInSeconds;
}

/// Узкая ошибка HTTP-слоя LLM Integrations.
///
/// Сознательно простая (один класс с полями), без иерархии подклассов:
/// UI различает кейсы по [errorCode] (`access_denied`/`invalid_state`/...)
/// и `statusCode`, а не по типу исключения.
class LlmIntegrationsException implements Exception {
  const LlmIntegrationsException({
    required this.message,
    this.statusCode,
    this.errorCode,
  });

  final String message;
  final int? statusCode;
  final String? errorCode;

  @override
  String toString() =>
      'LlmIntegrationsException(status=$statusCode, code=$errorCode, message=$message)';
}
