import 'package:freezed_annotation/freezed_annotation.dart';

part 'claude_code_status_model.freezed.dart';

/// Снимок статуса OAuth-подписки Claude Code (нормализованный из
/// `ClaudeCodeAuthStatus` settings-репозитория для использования в Integrations).
///
/// В отличие от settings-модели здесь нет полей `tokenType` / `scopes`,
/// которые UI экрана LLM Integrations не отображает — задача 2.4 SSOT:
/// «модели для экрана LLM Integrations», а не полный мирор бэк-DTO.
@freezed
abstract class ClaudeCodeIntegrationStatus with _$ClaudeCodeIntegrationStatus {
  const factory ClaudeCodeIntegrationStatus({
    required bool connected,
    DateTime? expiresAt,
    DateTime? lastRefreshedAt,
  }) = _ClaudeCodeIntegrationStatus;
}
