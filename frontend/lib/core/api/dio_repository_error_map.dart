import 'package:dio/dio.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_api_error.dart';

/// Единый разбор [DioException] для репозиториев (401 — [UnauthorizedException], остальное — фабрики).
///
/// [onMissingStatusCode] получает [isNetworkTransportError] — для репозиториев,
/// у которых базовое API-исключение различает транспортную ошибку и прочее
/// (см. [TaskApiException], [ConversationApiException]).
Exception mapDioExceptionForRepository(
  DioException error, {
  required Exception Function(String message, DioException original)
      onCancelled,
  required Exception Function(
    String message,
    DioException original,
    bool isNetworkTransportError,
  ) onMissingStatusCode,
  required Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  ) on401,
  required Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  ) on403,
  required Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  ) on404,
  Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  )? on409,
  Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  )? on422,
  Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
  )? on429,
  required Exception Function(
    String message,
    DioException original,
    String? apiErrorCode,
    int statusCode,
  ) onOtherHttp,
}) {
  final p = parseDioApiError(error);

  if (p.isCancellation) {
    return onCancelled(p.sanitizedMessage, error);
  }

  final statusCode = p.statusCode;
  if (statusCode == null) {
    return onMissingStatusCode(
      p.sanitizedMessage,
      error,
      p.isNetworkTransportError,
    );
  }

  switch (statusCode) {
    case 401:
      return on401(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
      );
    case 403:
      return on403(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
      );
    case 404:
      return on404(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
      );
    case 409:
      final map409 = on409;
      if (map409 != null) {
        return map409(
          p.sanitizedMessage,
          error,
          p.stableErrorCode,
        );
      }
      return onOtherHttp(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
        statusCode,
      );
    case 422:
      final map422 = on422;
      if (map422 != null) {
        return map422(
          p.sanitizedMessage,
          error,
          p.stableErrorCode,
        );
      }
      return onOtherHttp(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
        statusCode,
      );
    case 429:
      final map429 = on429;
      if (map429 != null) {
        return map429(
          p.sanitizedMessage,
          error,
          p.stableErrorCode,
        );
      }
      return onOtherHttp(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
        statusCode,
      );
    default:
      return onOtherHttp(
        p.sanitizedMessage,
        error,
        p.stableErrorCode,
        statusCode,
      );
  }
}

/// Делегат для [mapDioExceptionForRepository.on401] — общий [UnauthorizedException].
Exception unauthorizedFromDio(
  String message,
  DioException original,
  String? apiErrorCode,
) {
  return UnauthorizedException(
    message,
    originalError: original,
    apiErrorCode: apiErrorCode,
  );
}
