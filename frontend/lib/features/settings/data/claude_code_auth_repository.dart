import 'package:dio/dio.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';

/// Sprint 15.29 — HTTP-слой подписки Claude Code (15.12: /claude-code/auth/*).
class ClaudeCodeAuthRepository {
  ClaudeCodeAuthRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;
  static const _basePath = '/claude-code/auth';

  /// POST /claude-code/auth/init — старт device-flow.
  Future<ClaudeCodeAuthInit> initDeviceFlow() async {
    final resp = await _dio.post<Map<String, dynamic>>('$_basePath/init');
    return ClaudeCodeAuthInit.fromJson(resp.data!);
  }

  /// POST /claude-code/auth/callback — один шаг поллинга.
  ///
  /// Бэк может вернуть:
  ///   - 200: подписка успешно сохранена, возвращён статус.
  ///   - 202: authorization_pending — повторить через интервал.
  ///   - 410: expired_token / access_denied — поток завершён.
  /// Эту логику UI разбирает по [DioException.response?.statusCode].
  Future<ClaudeCodeAuthStatus> complete(String deviceCode) async {
    final resp = await _dio.post<Map<String, dynamic>>(
      '$_basePath/callback',
      data: <String, dynamic>{'device_code': deviceCode},
    );
    return ClaudeCodeAuthStatus.fromJson(resp.data!);
  }

  /// GET /claude-code/auth/status — текущий статус.
  Future<ClaudeCodeAuthStatus> status() async {
    final resp = await _dio.get<Map<String, dynamic>>('$_basePath/status');
    return ClaudeCodeAuthStatus.fromJson(resp.data!);
  }

  /// DELETE /claude-code/auth — отзыв подписки.
  Future<void> revoke() async {
    await _dio.delete<dynamic>(_basePath);
  }
}
