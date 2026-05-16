import 'package:freezed_annotation/freezed_annotation.dart';

part 'git_integration_model.freezed.dart';
part 'git_integration_model.g.dart';

/// Идентификаторы git-провайдеров (UI Refactoring §5 Этап 3).
///
/// SSOT: бэк держит соответствующие значения в `models.GitIntegrationProvider`
/// (`github`, `gitlab`). `gitlab` покрывает оба сценария — gitlab.com (Shared)
/// и self-hosted (BYO); вариант определяется наличием `host`.
enum GitIntegrationProvider {
  github('github'),
  gitlab('gitlab');

  const GitIntegrationProvider(this.jsonValue);

  /// Канонический идентификатор — совпадает с `events.IntegrationConnectionChanged.Provider`.
  final String jsonValue;

  static GitIntegrationProvider? tryParse(String raw) {
    for (final v in GitIntegrationProvider.values) {
      if (v.jsonValue == raw) {
        return v;
      }
    }
    return null;
  }
}

/// Состояние подключения git-провайдера для текущего пользователя.
///
/// Зеркало `events.IntegrationConnectionStatus`. UI-state `cancelled` различается
/// через [GitProviderConnection.reason] = `"user_cancelled"` (§4a.5).
enum GitProviderConnectionStatus { connected, disconnected, pending, error }

/// Снимок состояния одного git-провайдера на экране Git Integrations.
///
/// `host`/`accountLogin` приходят только когда `status == connected` (см.
/// `GitIntegrationStatusResponse` на бэке). `remoteRevokeFailed` — флаг из ответа
/// `DELETE /revoke`: остаётся в UI до следующего успешного `status` или WS-эха.
@freezed
abstract class GitProviderConnection with _$GitProviderConnection {
  const factory GitProviderConnection({
    required GitIntegrationProvider provider,
    required GitProviderConnectionStatus status,
    String? host,
    String? accountLogin,
    String? scopes,
    String? reason,
    DateTime? connectedAt,
    DateTime? expiresAt,
    @Default(false) bool remoteRevokeFailed,
  }) = _GitProviderConnection;

  factory GitProviderConnection.fromJson(Map<String, dynamic> json) =>
      _$GitProviderConnectionFromJson(json);
}
