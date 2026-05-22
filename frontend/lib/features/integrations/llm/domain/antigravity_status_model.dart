/// Снимок статуса OAuth-подписки Antigravity для использования в Integrations.
class AntigravityIntegrationStatus {
  const AntigravityIntegrationStatus({
    required this.connected,
    this.expiresAt,
    this.lastRefreshedAt,
  });

  final bool connected;
  final DateTime? expiresAt;
  final DateTime? lastRefreshedAt;

  @override
  bool operator ==(Object other) =>
      identical(this, other) ||
      other is AntigravityIntegrationStatus &&
          runtimeType == other.runtimeType &&
          connected == other.connected &&
          expiresAt == other.expiresAt &&
          lastRefreshedAt == other.lastRefreshedAt;

  @override
  int get hashCode =>
      connected.hashCode ^ expiresAt.hashCode ^ lastRefreshedAt.hashCode;
}

/// Данные device-flow для Antigravity.
class AntigravityOAuthInit {
  const AntigravityOAuthInit({
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
