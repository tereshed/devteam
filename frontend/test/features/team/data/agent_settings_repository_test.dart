// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/team/data/agent_settings_repository.dart';
import 'package:frontend/features/team/domain/agent_settings_exceptions.dart';
import 'package:mocktail/mocktail.dart';

class MockDio extends Mock implements Dio {}

class _RO extends Fake implements RequestOptions {}

void main() {
  setUpAll(() {
    registerFallbackValue(_RO());
  });

  group('AgentSettingsRepository (Sprint 15.M7/Major)', () {
    late MockDio dio;
    late AgentSettingsRepository repo;

    setUp(() {
      dio = MockDio();
      repo = AgentSettingsRepository(dio: dio);
    });

    test('get — happy path', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => Response<Map<String, dynamic>>(
                data: const <String, dynamic>{
                  'agent_id': '11111111-1111-1111-1111-111111111111',
                  'code_backend_settings': <String, dynamic>{},
                  'sandbox_permissions': <String, dynamic>{},
                },
                statusCode: 200,
                requestOptions: RequestOptions(path: '/agents/x/settings'),
              ));
      final s = await repo.get('11111111-1111-1111-1111-111111111111');
      expect(s.agentID, '11111111-1111-1111-1111-111111111111');
    });

    test('get — 404 → AgentSettingsNotFoundException', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/agents/x/settings'),
        response: Response(
          data: const {'error': 'agent_not_found'},
          statusCode: 404,
          requestOptions: RequestOptions(path: '/agents/x/settings'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.get('x'),
        throwsA(isA<AgentSettingsNotFoundException>()),
      );
    });

    test('update — 400 invalid body → AgentSettingsApiException', () async {
      when(() => dio.put<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/agents/x/settings'),
        response: Response(
          data: const {'error': 'bad_request'},
          statusCode: 400,
          requestOptions: RequestOptions(path: '/agents/x/settings'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.update('x',
            sandboxPermissions: const {'defaultMode': 'wrong'}),
        throwsA(isA<AgentSettingsApiException>()),
      );
    });

    test('update — 401 → UnauthorizedException (canon)', () async {
      when(() => dio.put<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/agents/x/settings'),
        response: Response(
          data: const {'error': 'access_denied'},
          statusCode: 401,
          requestOptions: RequestOptions(path: '/agents/x/settings'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.update('x'),
        throwsA(isA<UnauthorizedException>()),
      );
    });

    test('cancel → AgentSettingsCancelledException', () async {
      when(() => dio.get<Map<String, dynamic>>(any(),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/agents/x/settings'),
        type: DioExceptionType.cancel,
      ));
      await expectLater(
        () => repo.get('x'),
        throwsA(isA<AgentSettingsCancelledException>()),
      );
    });
  });
}
