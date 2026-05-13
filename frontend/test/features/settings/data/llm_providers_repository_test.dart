// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/features/settings/data/llm_providers_repository.dart';
import 'package:frontend/features/settings/domain/llm_providers_exceptions.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:mocktail/mocktail.dart';

class MockDio extends Mock implements Dio {}

class _RO extends Fake implements RequestOptions {}

void main() {
  setUpAll(() {
    registerFallbackValue(_RO());
  });

  group('LLMProvidersRepository (Sprint 15.M7/Major)', () {
    late MockDio dio;
    late LLMProvidersRepository repo;

    setUp(() {
      dio = MockDio();
      repo = LLMProvidersRepository(dio: dio);
    });

    test('list — happy path returns parsed providers', () async {
      when(() => dio.get<dynamic>(any(),
              queryParameters: any(named: 'queryParameters'),
              cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => Response<dynamic>(
                data: const [
                  {
                    'id': '11111111-1111-1111-1111-111111111111',
                    'name': 'OR prod',
                    'kind': 'openrouter',
                    'base_url': 'https://openrouter.ai/api/v1',
                    'auth_type': 'api_key',
                    'default_model': 'openrouter/auto',
                    'enabled': true,
                  }
                ],
                statusCode: 200,
                requestOptions: RequestOptions(path: '/llm-providers'),
              ));
      final list = await repo.list();
      expect(list, hasLength(1));
      expect(list.first, isA<LLMProviderModel>());
      expect(list.first.name, 'OR prod');
    });

    test('list — non-array response → LLMProvidersApiException', () async {
      when(() => dio.get<dynamic>(any(),
              queryParameters: any(named: 'queryParameters'),
              cancelToken: any(named: 'cancelToken')))
          .thenAnswer((_) async => Response<dynamic>(
                data: const {'oops': true},
                statusCode: 200,
                requestOptions: RequestOptions(path: '/llm-providers'),
              ));
      await expectLater(() => repo.list(), throwsA(isA<LLMProvidersApiException>()));
    });

    test('list — 401 → UnauthorizedException', () async {
      when(() => dio.get<dynamic>(any(),
              queryParameters: any(named: 'queryParameters'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers'),
        response: Response(
          data: const {'error': 'access_denied'},
          statusCode: 401,
          requestOptions: RequestOptions(path: '/llm-providers'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(() => repo.list(), throwsA(isA<UnauthorizedException>()));
    });

    test('list — 403 → LLMProvidersForbiddenException', () async {
      when(() => dio.get<dynamic>(any(),
              queryParameters: any(named: 'queryParameters'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers'),
        response: Response(
          data: const {'error': 'admin_only'},
          statusCode: 403,
          requestOptions: RequestOptions(path: '/llm-providers'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.list(),
        throwsA(isA<LLMProvidersForbiddenException>()),
      );
    });

    test('create — 409 → LLMProvidersConflictException', () async {
      when(() => dio.post<Map<String, dynamic>>(any(),
              data: any(named: 'data'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers'),
        response: Response(
          data: const {'error': 'llm_provider_name_exists'},
          statusCode: 409,
          requestOptions: RequestOptions(path: '/llm-providers'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.create(name: 'dup', kind: 'openrouter'),
        throwsA(isA<LLMProvidersConflictException>()),
      );
    });

    test('delete — 404 → LLMProvidersNotFoundException', () async {
      when(() => dio.delete<dynamic>(any(), cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers/x'),
        response: Response(
          data: const {'error': 'llm_provider_not_found'},
          statusCode: 404,
          requestOptions: RequestOptions(path: '/llm-providers/x'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.delete('x'),
        throwsA(isA<LLMProvidersNotFoundException>()),
      );
    });

    test('healthCheck — 502 → LLMProvidersApiException', () async {
      when(() => dio.post<dynamic>(any(), cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers/x/health-check'),
        response: Response(
          data: const {'error': 'health_check_failed'},
          statusCode: 502,
          requestOptions: RequestOptions(path: '/llm-providers/x/health-check'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.healthCheck('x'),
        throwsA(isA<LLMProvidersApiException>()),
      );
    });

    test('cancel → LLMProvidersCancelledException', () async {
      when(() => dio.get<dynamic>(any(),
              queryParameters: any(named: 'queryParameters'),
              cancelToken: any(named: 'cancelToken')))
          .thenThrow(DioException(
        requestOptions: RequestOptions(path: '/llm-providers'),
        type: DioExceptionType.cancel,
      ));
      await expectLater(
        () => repo.list(),
        throwsA(isA<LLMProvidersCancelledException>()),
      );
    });
  });
}
