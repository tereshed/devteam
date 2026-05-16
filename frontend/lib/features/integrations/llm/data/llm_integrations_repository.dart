import 'package:dio/dio.dart';
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
      case LlmIntegrationProvider.claudeCodeOAuth:
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
      case LlmIntegrationProvider.claudeCodeOAuth:
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
