import 'package:freezed_annotation/freezed_annotation.dart';

part 'claude_code_auth_status.freezed.dart';
part 'claude_code_auth_status.g.dart';

/// Sprint 15.28 — статус OAuth-подписки Claude Code (GET /claude-code/auth/status).
@freezed
abstract class ClaudeCodeAuthStatus with _$ClaudeCodeAuthStatus {
  const factory ClaudeCodeAuthStatus({
    required bool connected,
    @JsonKey(name: 'token_type') @Default('') String tokenType,
    @Default('') String scopes,
    @JsonKey(name: 'expires_at') DateTime? expiresAt,
    @JsonKey(name: 'last_refreshed_at') DateTime? lastRefreshedAt,
  }) = _ClaudeCodeAuthStatus;

  factory ClaudeCodeAuthStatus.fromJson(Map<String, dynamic> json) =>
      _$ClaudeCodeAuthStatusFromJson(json);
}

/// Ответ POST /claude-code/auth/init — данные device-flow для UI.
@freezed
abstract class ClaudeCodeAuthInit with _$ClaudeCodeAuthInit {
  const factory ClaudeCodeAuthInit({
    @JsonKey(name: 'device_code') required String deviceCode,
    @JsonKey(name: 'user_code') required String userCode,
    @JsonKey(name: 'verification_uri') required String verificationURI,
    @JsonKey(name: 'verification_uri_complete')
    @Default('')
    String verificationURIComplete,
    @JsonKey(name: 'interval_seconds') @Default(5) int intervalSeconds,
    @JsonKey(name: 'expires_in_seconds') @Default(900) int expiresInSeconds,
  }) = _ClaudeCodeAuthInit;

  factory ClaudeCodeAuthInit.fromJson(Map<String, dynamic> json) =>
      _$ClaudeCodeAuthInitFromJson(json);
}
