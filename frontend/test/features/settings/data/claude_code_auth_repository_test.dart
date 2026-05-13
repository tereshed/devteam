// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/settings/data/claude_code_auth_repository.dart';
import 'package:frontend/features/settings/domain/claude_code_auth_exceptions.dart';
import 'package:mocktail/mocktail.dart';

class MockDio extends Mock implements Dio {}

class _RO extends Fake implements RequestOptions {}

void main() {
  setUpAll(() {
    registerFallbackValue(_RO());
  });

  group('ClaudeCodeAuthRepository (Sprint 15.M7)', () {
    late MockDio dio;
    late ClaudeCodeAuthRepository repo;

    setUp(() {
      dio = MockDio();
      repo = ClaudeCodeAuthRepository(dio: dio);
    });

    test('status — happy path', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => Response<Map<String, dynamic>>(
                data: const {'connected': true, 'token_type': 'Bearer'},
                statusCode: 200,
                requestOptions: RequestOptions(path: '/claude-code/auth/status'),
              ));
      final s = await repo.status();
      expect(s.connected, isTrue);
      expect(s.tokenType, 'Bearer');
    });

    test('complete — 202 → ClaudeCodeAuthorizationPendingException', () async {
      when(() => dio.post<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => Response<Map<String, dynamic>>(
                data: const {},
                statusCode: 202,
                requestOptions: RequestOptions(path: '/cb'),
              ));
      await expectLater(
        () => repo.complete('dc'),
        throwsA(isA<ClaudeCodeAuthorizationPendingException>()),
      );
    });

    test('complete — 403 device_code_owner_mismatch → OwnerMismatch', () async {
      when(() => dio.post<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/cb'),
        response: Response(
          data: const {'error': 'device_code_owner_mismatch', 'details': 'not your code'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/cb'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.complete('dc'),
        throwsA(isA<ClaudeCodeAuthOwnerMismatchException>()),
      );
    });

    test('callback — 410 → ClaudeCodeAuthFlowEndedException', () async {
      when(() => dio.post<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/cb'),
        response: Response(
          data: const {'error': 'expired_token'},
          statusCode: 410,
          requestOptions: RequestOptions(path: '/cb'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.complete('dc'),
        throwsA(isA<ClaudeCodeAuthFlowEndedException>()),
      );
    });

    test('any — 401 → UnauthorizedException (canon)', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/status'),
        response: Response(
          data: const {'error': 'access_denied'},
          statusCode: 401,
          requestOptions: RequestOptions(path: '/status'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.status(),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('any — cancel → ClaudeCodeAuthCancelledException', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/status'),
        type: DioExceptionType.cancel,
      ));
      await expectLater(
        () => repo.status(),
        throwsA(isA<ClaudeCodeAuthCancelledException>()),
      );
    });
  });
}
