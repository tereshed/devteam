import 'package:freezed_annotation/freezed_annotation.dart';

part 'llm_provider_model.freezed.dart';

/// Идентификаторы LLM-провайдеров для экрана LLM Integrations.
///
/// SSOT: бэк держит соответствующие провайдеры в `models.UserLLMProvider`
/// и `service.ProviderClaudeCodeOAuth` (claude_code_oauth). Этот enum нужен
/// только для UI-маппинга: иконка/название/CTA → конкретный API-вызов.
enum LlmIntegrationProvider {
  /// OAuth-подписка Claude Code (особый flow через device-code).
  claudeCodeOAuth('claude_code_oauth'),
  anthropic('anthropic'),
  openai('openai'),
  deepseek('deepseek'),
  openrouter('openrouter'),
  zhipu('zhipu'),
  gemini('gemini'),
  qwen('qwen');

  const LlmIntegrationProvider(this.jsonValue);

  /// Канонический идентификатор (совпадает с `events.IntegrationConnectionChanged.Provider`
  /// и с ключами `/me/llm-credentials`).
  final String jsonValue;

  static LlmIntegrationProvider? tryParse(String raw) {
    for (final v in LlmIntegrationProvider.values) {
      if (v.jsonValue == raw) {
        return v;
      }
    }
    return null;
  }
}

/// Состояние подключения LLM-провайдера для текущего пользователя.
///
/// Маппинг с `events.IntegrationConnectionStatus`: `connected/disconnected/error/pending`.
/// UI-стейт `cancelled` различается через [LlmProviderConnection.reason] = `"user_cancelled"`,
/// см. dashboard-redesign §4a.5.
enum LlmProviderConnectionStatus {
  connected,
  disconnected,
  pending,
  error,
}

/// Снимок состояния одного провайдера на экране LLM Integrations.
///
/// `maskedPreview` приходит только для API-key провайдеров (см. `LlmCredentialsResponse`
/// на бэке). Для Claude Code OAuth это поле всегда null (вместо него expiresAt/scopes).
@freezed
abstract class LlmProviderConnection with _$LlmProviderConnection {
  const factory LlmProviderConnection({
    required LlmIntegrationProvider provider,
    required LlmProviderConnectionStatus status,
    String? maskedPreview,
    String? reason,
    DateTime? connectedAt,
    DateTime? expiresAt,
  }) = _LlmProviderConnection;
}
