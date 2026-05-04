import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_api_error.dart';

void main() {
  group('requireResponseJsonMap', () {
    test('возвращает map при валидном теле', () {
      final r = Response<dynamic>(
        data: <String, dynamic>{'a': 1},
        statusCode: 200,
        requestOptions: RequestOptions(path: '/x'),
      );
      final m = requireResponseJsonMap(
        r,
        onInvalid: (_, __) => throw AssertionError('onInvalid'),
      );
      expect(m, <String, dynamic>{'a': 1});
    });

    test('onInvalid при null data', () {
      final r = Response<dynamic>(
        data: null,
        statusCode: 200,
        requestOptions: RequestOptions(path: '/x'),
      );
      expect(
        () => requireResponseJsonMap(
          r,
          onInvalid: (msg, code) {
            expect(msg, 'Empty response body');
            expect(code, 200);
            throw StateError('stop');
          },
        ),
        throwsStateError,
      );
    });

    test('onInvalid при не-Map теле', () {
      final r = Response<dynamic>(
        data: 'plain',
        statusCode: 200,
        requestOptions: RequestOptions(path: '/x'),
      );
      expect(
        () => requireResponseJsonMap(
          r,
          onInvalid: (msg, code) {
            expect(msg, 'Expected JSON object in response body');
            expect(code, 200);
            throw StateError('stop');
          },
        ),
        throwsStateError,
      );
    });
  });

  group('parseDioApiError', () {
    test('connectionTimeout sets isNetworkTransportError', () {
      final p = parseDioApiError(
        DioException(
          requestOptions: RequestOptions(path: '/a'),
          type: DioExceptionType.connectionTimeout,
        ),
      );
      expect(p.isNetworkTransportError, isTrue);
      expect(p.statusCode, isNull);
    });

    test('connectionError sets isNetworkTransportError', () {
      final p = parseDioApiError(
        DioException(
          requestOptions: RequestOptions(path: '/a'),
          type: DioExceptionType.connectionError,
        ),
      );
      expect(p.isNetworkTransportError, isTrue);
    });

    test('badResponse leaves isNetworkTransportError false', () {
      final p = parseDioApiError(
        DioException(
          requestOptions: RequestOptions(path: '/a'),
          type: DioExceptionType.badResponse,
          response: Response<dynamic>(
            statusCode: 500,
            requestOptions: RequestOptions(path: '/a'),
            data: <String, dynamic>{'message': 'x'},
          ),
        ),
      );
      expect(p.isNetworkTransportError, isFalse);
      expect(p.statusCode, 500);
    });
  });
}
