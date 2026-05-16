// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

class _RO extends Fake implements RequestOptions {}

void main() {
  setUpAll(() {
    registerFallbackValue(_RO());
  });

  group('LlmIntegrationsRepository', () {
    late _MockDio dio;
    late LlmIntegrationsRepository repo;

    setUp(() {
      dio = _MockDio();
      repo = LlmIntegrationsRepository(dio: dio);
    });

    test('fetchApiKeyConnections — parses masked previews, marks empty as disconnected',
        () async {
      when(() => dio.get<Map<String, dynamic>>(
            '/me/llm-credentials',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{
              'anthropic': {'masked_preview': '****3Mk9'},
              'openai': {'masked_preview': null},
              'deepseek': {'masked_preview': '****xyzA'},
              'openrouter': {'masked_preview': null},
              'gemini': {'masked_preview': null},
              'qwen': {'masked_preview': null},
            },
            statusCode: 200,
            requestOptions: RequestOptions(path: '/me/llm-credentials'),
          ));

      final result = await repo.fetchApiKeyConnections();
      final byProvider = {for (final c in result) c.provider: c};

      expect(byProvider[LlmIntegrationProvider.anthropic]?.status,
          LlmProviderConnectionStatus.connected);
      expect(byProvider[LlmIntegrationProvider.anthropic]?.maskedPreview,
          '****3Mk9');
      expect(byProvider[LlmIntegrationProvider.openai]?.status,
          LlmProviderConnectionStatus.disconnected);
      expect(byProvider[LlmIntegrationProvider.deepseek]?.maskedPreview,
          '****xyzA');
    });

    test('fetchClaudeCodeStatus — parses connected and expiresAt', () async {
      when(() => dio.get<Map<String, dynamic>>(
            '/claude-code/auth/status',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{
              'connected': true,
              'expires_at': '2030-01-01T00:00:00Z',
            },
            statusCode: 200,
            requestOptions: RequestOptions(path: '/claude-code/auth/status'),
          ));
      final s = await repo.fetchClaudeCodeStatus();
      expect(s.connected, isTrue);
      expect(s.expiresAt, isNotNull);
      expect(s.expiresAt!.isUtc, isTrue);
    });

    test('setApiKey — анthropic — патчит anthropic_api_key', () async {
      when(() => dio.patch<Map<String, dynamic>>(
            '/me/llm-credentials',
            data: any(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{},
            statusCode: 200,
            requestOptions: RequestOptions(path: '/me/llm-credentials'),
          ));

      await repo.setApiKey(
        provider: LlmIntegrationProvider.anthropic,
        apiKey: 'sk-ant-XXXX',
      );

      final captured = verify(() => dio.patch<Map<String, dynamic>>(
            '/me/llm-credentials',
            data: captureAny(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).captured.single as Map<String, dynamic>;
      expect(captured['anthropic_api_key'], 'sk-ant-XXXX');
    });

    test('clearApiKey — openai — патчит clear_openai_key=true', () async {
      when(() => dio.patch<Map<String, dynamic>>(
            '/me/llm-credentials',
            data: any(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{},
            statusCode: 200,
            requestOptions: RequestOptions(path: '/me/llm-credentials'),
          ));

      await repo.clearApiKey(provider: LlmIntegrationProvider.openai);

      final captured = verify(() => dio.patch<Map<String, dynamic>>(
            '/me/llm-credentials',
            data: captureAny(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).captured.single as Map<String, dynamic>;
      expect(captured['clear_openai_key'], isTrue);
    });

    test('setApiKey — claudeCodeOAuth → ArgumentError', () {
      expect(
        () => repo.setApiKey(
          provider: LlmIntegrationProvider.claudeCodeOAuth,
          apiKey: 'x',
        ),
        throwsA(isA<ArgumentError>()),
      );
    });

    test('initClaudeCodeOAuth — happy path', () async {
      when(() => dio.post<Map<String, dynamic>>(
            '/claude-code/auth/init',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{
              'device_code': 'dc-1',
              'user_code': 'ABCD-EFGH',
              'verification_uri': 'https://login.example/device',
              'verification_uri_complete': 'https://login.example/device?dc=dc-1',
              'interval_seconds': 5,
              'expires_in_seconds': 600,
            },
            statusCode: 200,
            requestOptions: RequestOptions(path: '/claude-code/auth/init'),
          ));
      final init = await repo.initClaudeCodeOAuth();
      expect(init.deviceCode, 'dc-1');
      expect(init.userCode, 'ABCD-EFGH');
      expect(init.intervalSeconds, 5);
    });

    test('completeClaudeCodeOAuth — 202 → LlmIntegrationsException(authorization_pending)',
        () async {
      when(() => dio.post<Map<String, dynamic>>(
            '/claude-code/auth/callback',
            data: any(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<Map<String, dynamic>>(
            data: const <String, dynamic>{},
            statusCode: 202,
            requestOptions: RequestOptions(path: '/claude-code/auth/callback'),
          ));
      await expectLater(
        () => repo.completeClaudeCodeOAuth(deviceCode: 'dc'),
        throwsA(
          isA<LlmIntegrationsException>().having(
            (e) => e.errorCode,
            'errorCode',
            'authorization_pending',
          ),
        ),
      );
    });

    test('completeClaudeCodeOAuth — 410 access_denied → LlmIntegrationsException',
        () async {
      when(() => dio.post<Map<String, dynamic>>(
            '/claude-code/auth/callback',
            data: any(named: 'data'),
            cancelToken: any(named: 'cancelToken'),
          )).thenThrow(DioException(
        requestOptions: RequestOptions(path: '/claude-code/auth/callback'),
        response: Response(
          data: const <String, dynamic>{
            'error_code': 'access_denied',
            'message': 'User denied authorization',
          },
          statusCode: 410,
          requestOptions: RequestOptions(path: '/claude-code/auth/callback'),
        ),
        type: DioExceptionType.badResponse,
      ));
      await expectLater(
        () => repo.completeClaudeCodeOAuth(deviceCode: 'dc'),
        throwsA(
          isA<LlmIntegrationsException>()
              .having((e) => e.errorCode, 'errorCode', 'access_denied')
              .having((e) => e.statusCode, 'statusCode', 410),
        ),
      );
    });

    test('revokeClaudeCodeOAuth — passes through', () async {
      when(() => dio.delete<dynamic>(
            '/claude-code/auth',
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => Response<dynamic>(
            requestOptions: RequestOptions(path: '/claude-code/auth'),
            statusCode: 204,
          ));
      await repo.revokeClaudeCodeOAuth();
      verify(() => dio.delete<dynamic>(
            '/claude-code/auth',
            cancelToken: any(named: 'cancelToken'),
          )).called(1);
    });
  });
}
