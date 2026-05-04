import 'package:dio/dio.dart';
import 'package:frontend/core/utils/sanitize_user_facing_message.dart';

/// Нейтральное описание ошибки ответа API после разбора тела и санитизации.
///
/// Маппинг в feature-исключения (`ProjectNotFoundException`, `ConversationNotFoundException` и т.д.)
/// выполняется только в репозиториях.
///
/// [stableErrorCode] — только из JSON поля `error` при [badResponse]; не смешивать с [isCancellation].
///
/// [isCancellation] — **только** [DioExceptionType.cancel], чтобы не коллидировать с API-кодом
/// `"cancelled"` в теле ответа.
class ApiErrorPayload {
  const ApiErrorPayload({
    required this.sanitizedMessage,
    required this.requestPath,
    this.statusCode,
    this.stableErrorCode,
    this.isCancellation = false,
  });

  final int? statusCode;
  final String sanitizedMessage;
  final String? stableErrorCode;
  final String requestPath;
  final bool isCancellation;
}

/// Требует JSON-объект в [Response.data]. Иначе вызывает [onInvalid] (репозиторий кидает своё исключение).
///
/// [onInvalid] должен иметь возвращаемый тип [Never]. Возврат использует явный `as` на случай
/// рефакторинга сигнатуры (см. комментарий у `return`).
Map<String, dynamic> requireResponseJsonMap(
  Response<dynamic> response, {
  required Never Function(String reason, int? statusCode) onInvalid,
}) {
  final raw = response.data;
  if (raw == null) {
    onInvalid('Empty response body', response.statusCode);
  }
  if (raw is! Map<String, dynamic>) {
    onInvalid('Expected JSON object in response body', response.statusCode);
  }
  // Явный cast: при смене [onInvalid] с [Never] на обычный void promotion исчезнет; cast даст CastError.
  // ignore: unnecessary_cast
  return raw as Map<String, dynamic>;
}

String? _firstNonEmptyApiString(Map<String, dynamic> data, String key) {
  final v = data[key];
  if (v is! String) {
    return null;
  }
  final t = v.trim();
  return t.isEmpty ? null : t;
}

/// Разбор [DioException] в [ApiErrorPayload] без знания о фичах.
///
/// Тексты для пользователя всегда проходят через [sanitizeUserFacingMessage].
/// Для ответа API без человекочитаемого `message` используется нейтральный fallback,
/// стабильный код при этом остаётся только в [ApiErrorPayload.stableErrorCode].
ApiErrorPayload parseDioApiError(DioException error) {
  final requestPath = error.requestOptions.path;

  switch (error.type) {
    case DioExceptionType.badResponse:
      final statusCode = error.response?.statusCode;
      final data = error.response?.data;

      String raw;
      String? stableCode;
      if (data is Map<String, dynamic>) {
        final err = data['error'];
        if (err is String && err.trim().isNotEmpty) {
          stableCode = err.trim();
        }
        raw = _firstNonEmptyApiString(data, 'message') ?? 'Request failed';
      } else {
        raw = data?.toString() ?? 'Request failed';
      }

      return ApiErrorPayload(
        statusCode: statusCode,
        stableErrorCode: stableCode,
        sanitizedMessage: sanitizeUserFacingMessage(raw),
        requestPath: requestPath,
      );

    case DioExceptionType.connectionTimeout:
    case DioExceptionType.receiveTimeout:
    case DioExceptionType.sendTimeout:
      return ApiErrorPayload(
        statusCode: null,
        stableErrorCode: null,
        sanitizedMessage: sanitizeUserFacingMessage('Network timeout'),
        requestPath: requestPath,
      );

    case DioExceptionType.connectionError:
      return ApiErrorPayload(
        statusCode: null,
        stableErrorCode: null,
        sanitizedMessage: sanitizeUserFacingMessage('Network error'),
        requestPath: requestPath,
      );

    case DioExceptionType.cancel:
      return ApiErrorPayload(
        statusCode: null,
        stableErrorCode: null,
        sanitizedMessage: sanitizeUserFacingMessage('Request cancelled'),
        requestPath: requestPath,
        isCancellation: true,
      );

    default:
      return ApiErrorPayload(
        statusCode: null,
        stableErrorCode: null,
        sanitizedMessage: sanitizeUserFacingMessage(
          error.message ?? 'Unknown error',
        ),
        requestPath: requestPath,
      );
  }
}
