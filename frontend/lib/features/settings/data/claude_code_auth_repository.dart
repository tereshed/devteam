import 'package:dio/dio.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';
import 'package:frontend/features/settings/domain/claude_code_auth_exceptions.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';

/// Sprint 15.29 / 15.B (F9) — HTTP-слой подписки Claude Code.
/// Использует канонический [mapDioExceptionForRepository] + бизнес-коды device-flow
/// (202 pending, 410 expired, 403 owner-mismatch).
class ClaudeCodeAuthRepository {
  ClaudeCodeAuthRepository({required Dio dio}) : _dio = dio;

  final Dio _dio;
  static const _basePath = '/claude-code/auth';

  Exception _mapError(DioException error) {
    return mapDioExceptionForRepository(
      error,
      onCancelled: (msg, err) =>
          ClaudeCodeAuthCancelledException(msg, originalError: err),
      onMissingStatusCode: (msg, err, _) =>
          ClaudeCodeAuthApiException(msg, originalError: err),
      on401: unauthorizedFromDio,
      on403: (msg, err, code) {
        // Sprint 15.B (B2) backend mapping: device_code_owner_mismatch.
        if (code == 'device_code_owner_mismatch') {
          return ClaudeCodeAuthOwnerMismatchException(msg, originalError: err);
        }
        return ClaudeCodeAuthForbiddenException(msg,
            originalError: err, apiErrorCode: code);
      },
      on404: (msg, err, code) => ClaudeCodeAuthNotFoundException(msg,
          originalError: err, apiErrorCode: code),
      on409: (msg, err, code) => ClaudeCodeAuthConflictException(msg,
          originalError: err, apiErrorCode: code),
      on429: (msg, err, _) =>
          ClaudeCodeAuthSlowDownException(msg, originalError: err),
      onOtherHttp: (msg, err, code, status) {
        // 202 authorization_pending: бэк отдаёт apierror с body, Dio расценивает <300 как OK,
        // поэтому 202 НЕ попадает сюда (см. complete() — там специальный путь).
        if (status == 410) {
          return ClaudeCodeAuthFlowEndedException(msg,
              originalError: err, apiErrorCode: code);
        }
        return ClaudeCodeAuthApiException(msg,
            statusCode: status, originalError: err, apiErrorCode: code);
      },
    );
  }

  /// POST /claude-code/auth/init — старт device-flow.
  Future<ClaudeCodeAuthInit> initDeviceFlow({CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '$_basePath/init',
        cancelToken: cancelToken,
      );
      return ClaudeCodeAuthInit.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  /// POST /claude-code/auth/callback — один шаг поллинга.
  ///
  /// Sprint 15.B (F9): отдельный путь для статуса 202 (authorization_pending).
  /// Dio по умолчанию считает 202 успехом (<300), и нужно явно проверить statusCode
  /// перед попыткой парсинга тела как ClaudeCodeAuthStatus.
  Future<ClaudeCodeAuthStatus> complete(
    String deviceCode, {
    CancelToken? cancelToken,
  }) async {
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '$_basePath/callback',
        data: <String, dynamic>{'device_code': deviceCode},
        cancelToken: cancelToken,
      );
      if (resp.statusCode == 202) {
        throw ClaudeCodeAuthorizationPendingException(
          'authorization_pending',
        );
      }
      return ClaudeCodeAuthStatus.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<ClaudeCodeAuthStatus> status({CancelToken? cancelToken}) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '$_basePath/status',
        cancelToken: cancelToken,
      );
      return ClaudeCodeAuthStatus.fromJson(resp.data!);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }

  Future<void> revoke({CancelToken? cancelToken}) async {
    try {
      await _dio.delete<dynamic>(_basePath, cancelToken: cancelToken);
    } on DioException catch (e) {
      throw _mapError(e);
    }
  }
}
