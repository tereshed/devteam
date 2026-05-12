import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/dio_repository_error_map.dart';

/// Маркеры, чтобы не плодить feature-specific исключения в тестах самого хелпера.
class _MarkerException implements Exception {
  _MarkerException(
    this.tag, {
    this.statusCode,
    this.apiErrorCode,
    this.isNetworkTransportError = false,
    this.original,
  });
  final String tag;
  final int? statusCode;
  final String? apiErrorCode;
  final bool isNetworkTransportError;
  final DioException? original;
}

DioException _badResponse({
  required int status,
  String? errorCode,
  String? message,
  String path = '/x',
}) {
  return DioException(
    requestOptions: RequestOptions(path: path),
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      statusCode: status,
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        if (errorCode != null) 'error': errorCode,
        if (message != null) 'message': message,
      },
    ),
  );
}

Exception _map(
  DioException error, {
  Exception Function(String, DioException, String?)? on409,
  Exception Function(String, DioException, String?)? on422,
  Exception Function(String, DioException, String?)? on429,
}) {
  return mapDioExceptionForRepository(
    error,
    onCancelled: (msg, err) => _MarkerException('cancelled', original: err),
    onMissingStatusCode: (msg, err, isTransport) => _MarkerException(
      'missing',
      isNetworkTransportError: isTransport,
      original: err,
    ),
    on401: (msg, err, code) => UnauthorizedException(
      msg,
      originalError: err,
      apiErrorCode: code,
    ),
    on403: (msg, err, code) => _MarkerException(
      '403',
      apiErrorCode: code,
      original: err,
    ),
    on404: (msg, err, code) => _MarkerException(
      '404@${err.requestOptions.path}',
      apiErrorCode: code,
      original: err,
    ),
    on409: on409,
    on422: on422,
    on429: on429,
    onOtherHttp: (msg, err, code, status) => _MarkerException(
      'other',
      statusCode: status,
      apiErrorCode: code,
      original: err,
    ),
  );
}

void main() {
  group('mapDioExceptionForRepository', () {
    test('cancel → onCancelled', () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.cancel,
      );
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'cancelled');
    });

    test('connectionTimeout → onMissingStatusCode с isNetworkTransportError=true',
        () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.connectionTimeout,
      );
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'missing');
      expect(ex.isNetworkTransportError, isTrue);
    });

    test('connectionError → onMissingStatusCode с isNetworkTransportError=true',
        () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.connectionError,
      );
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'missing');
      expect(ex.isNetworkTransportError, isTrue);
    });

    test('unknown без response → onMissingStatusCode без транспортного флага', () {
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.unknown,
      );
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'missing');
      expect(ex.isNetworkTransportError, isFalse);
    });

    test('401 → on401 (UnauthorizedException, stableCode пробрасывается)', () {
      final e = _badResponse(status: 401, errorCode: 'token_expired');
      final ex = _map(e);
      expect(ex, isA<UnauthorizedException>());
      expect((ex as UnauthorizedException).apiErrorCode, 'token_expired');
    });

    test('403 → on403', () {
      final e = _badResponse(status: 403, errorCode: 'forbidden');
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, '403');
      expect(ex.apiErrorCode, 'forbidden');
    });

    test('404 → on404 (path виден через DioException.requestOptions.path)', () {
      final e = _badResponse(status: 404, path: '/projects/abc/tasks');
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, '404@/projects/abc/tasks');
    });

    test('409 с on409=null → onOtherHttp со statusCode == 409', () {
      final e = _badResponse(status: 409, errorCode: 'conflict_state');
      final ex = _map(e, on409: null) as _MarkerException;
      expect(ex.tag, 'other');
      expect(ex.statusCode, 409);
      expect(ex.apiErrorCode, 'conflict_state');
    });

    test('409 с on409 задан → on409', () {
      final e = _badResponse(status: 409, errorCode: 'conflict');
      final ex = _map(
        e,
        on409: (msg, err, code) => _MarkerException('409', apiErrorCode: code),
      ) as _MarkerException;
      expect(ex.tag, '409');
      expect(ex.apiErrorCode, 'conflict');
    });

    test('422 с on422=null → onOtherHttp со statusCode == 422', () {
      final e = _badResponse(status: 422);
      final ex = _map(e, on422: null) as _MarkerException;
      expect(ex.tag, 'other');
      expect(ex.statusCode, 422);
    });

    test('422 с on422 задан → on422', () {
      final e = _badResponse(status: 422, errorCode: 'invalid_state');
      final ex = _map(
        e,
        on422: (msg, err, code) => _MarkerException('422', apiErrorCode: code),
      ) as _MarkerException;
      expect(ex.tag, '422');
      expect(ex.apiErrorCode, 'invalid_state');
    });

    test('429 с on429=null → onOtherHttp со statusCode == 429', () {
      final e = _badResponse(status: 429);
      final ex = _map(e, on429: null) as _MarkerException;
      expect(ex.tag, 'other');
      expect(ex.statusCode, 429);
    });

    test('429 с on429 задан → on429', () {
      final e = _badResponse(status: 429, errorCode: 'rate_limited');
      final ex = _map(
        e,
        on429: (msg, err, code) => _MarkerException('429', apiErrorCode: code),
      ) as _MarkerException;
      expect(ex.tag, '429');
      expect(ex.apiErrorCode, 'rate_limited');
    });

    test('500 (произвольный) → onOtherHttp', () {
      final e = _badResponse(status: 500, errorCode: 'internal');
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'other');
      expect(ex.statusCode, 500);
      expect(ex.apiErrorCode, 'internal');
    });

    test('sanitizedMessage прокидывается в фабрики', () {
      final e = _badResponse(
        status: 500,
        message: 'Backend explosion',
      );
      // через onOtherHttp → 500
      final ex = mapDioExceptionForRepository(
        e,
        onCancelled: (msg, err) => _MarkerException('cancelled'),
        onMissingStatusCode: (msg, err, _) => _MarkerException('missing'),
        on401: (msg, err, code) => UnauthorizedException(msg),
        on403: (msg, err, code) => _MarkerException(msg),
        on404: (msg, err, code) => _MarkerException(msg),
        onOtherHttp: (msg, err, code, status) =>
            _MarkerException(msg, statusCode: status),
      ) as _MarkerException;
      expect(ex.tag, contains('Backend explosion'));
      expect(ex.statusCode, 500);
    });

    test('cancel приоритетнее любого statusCode в response', () {
      // cancel-тип Dio + случайный response (бывает в Dio при cancel в interceptors).
      final e = DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.cancel,
        response: Response<dynamic>(
          statusCode: 500,
          requestOptions: RequestOptions(path: '/x'),
        ),
      );
      final ex = _map(e) as _MarkerException;
      expect(ex.tag, 'cancelled');
    });
  });

  group('unauthorizedFromDio', () {
    test('создаёт UnauthorizedException с apiErrorCode', () {
      final e = _badResponse(status: 401, errorCode: 'token_expired');
      final ex = unauthorizedFromDio('msg', e, 'token_expired')
          as UnauthorizedException;
      expect(ex.apiErrorCode, 'token_expired');
      expect(ex.originalError, e);
    });
  });
}
