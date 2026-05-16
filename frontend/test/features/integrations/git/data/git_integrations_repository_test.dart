// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_repository.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:mocktail/mocktail.dart';

class _MockDio extends Mock implements Dio {}

class _RO extends Fake implements RequestOptions {}

void main() {
  setUpAll(() {
    registerFallbackValue(_RO());
  });

  group('GitIntegrationsRepository', () {
    late _MockDio dio;
    late GitIntegrationsRepository repo;

    setUp(() {
      dio = _MockDio();
      repo = GitIntegrationsRepository(dio: dio);
    });

    test('fetchStatus — parses connected + host + connected_at', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/integrations/github/auth/status',
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{
            'provider': 'github',
            'connected': true,
            'account_login': 'octocat',
            'scopes': 'repo,read:user',
            'connected_at': '2030-01-01T00:00:00Z',
            'expires_at': '2030-06-01T00:00:00Z',
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/github/auth/status',
          ),
        ),
      );

      final s = await repo.fetchStatus(GitIntegrationProvider.github);

      expect(s.status, GitProviderConnectionStatus.connected);
      expect(s.accountLogin, 'octocat');
      expect(s.scopes, 'repo,read:user');
      expect(s.connectedAt?.isUtc, isTrue);
      expect(s.expiresAt?.isUtc, isTrue);
    });

    test('fetchStatus — connected=false → disconnected', () async {
      when(
        () => dio.get<Map<String, dynamic>>(
          '/integrations/gitlab/auth/status',
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{'connected': false},
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/gitlab/auth/status',
          ),
        ),
      );

      final s = await repo.fetchStatus(GitIntegrationProvider.gitlab);

      expect(s.status, GitProviderConnectionStatus.disconnected);
      expect(s.accountLogin, isNull);
    });

    test('init — github sends redirect_uri only', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/github/auth/init',
          data: any(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{
            'authorize_url': 'https://github.com/login/oauth/authorize?x=1',
            'state': 'st-1',
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/github/auth/init',
          ),
        ),
      );

      final init = await repo.init(
        GitIntegrationProvider.github,
        redirectUri: 'https://example.app/cb',
      );
      expect(init.authorizeUrl, contains('github.com'));
      expect(init.state, 'st-1');

      final captured = verify(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/github/auth/init',
          data: captureAny(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).captured.single as Map<String, dynamic>;
      expect(captured['redirect_uri'], 'https://example.app/cb');
      expect(captured.containsKey('host'), isFalse);
      expect(captured.containsKey('byo_client_id'), isFalse);
      expect(captured.containsKey('byo_client_secret'), isFalse);
    });

    test('init — gitlab BYO passes host + byo creds', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/gitlab/auth/init',
          data: any(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{
            'authorize_url': 'https://gl.acme.corp/oauth/authorize?x=1',
            'state': 'st-2',
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/gitlab/auth/init',
          ),
        ),
      );

      await repo.init(
        GitIntegrationProvider.gitlab,
        redirectUri: 'https://example.app/cb',
        host: 'https://gl.acme.corp',
        byoClientId: 'cid',
        byoClientSecret: 'secret',
      );

      final captured = verify(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/gitlab/auth/init',
          data: captureAny(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).captured.single as Map<String, dynamic>;
      expect(captured['host'], 'https://gl.acme.corp');
      expect(captured['byo_client_id'], 'cid');
      expect(captured['byo_client_secret'], 'secret');
    });

    test('completeCallback — parses connected status from response', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/github/auth/callback',
          data: any(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{
            'provider': 'github',
            'status': {
              'provider': 'github',
              'connected': true,
              'account_login': 'octocat',
            },
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/github/auth/callback',
          ),
        ),
      );

      final s = await repo.completeCallback(
        GitIntegrationProvider.github,
        code: 'c',
        state: 'st',
      );
      expect(s.status, GitProviderConnectionStatus.connected);
      expect(s.accountLogin, 'octocat');
    });

    test('init — 400 invalid_host → GitIntegrationsException', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/gitlab/auth/init',
          data: any(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/integrations/gitlab/auth/init',
          ),
          response: Response(
            data: const <String, dynamic>{
              'error_code': 'invalid_host',
              'message': 'Provided git host is not allowed',
            },
            statusCode: 400,
            requestOptions: RequestOptions(
              path: '/integrations/gitlab/auth/init',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      await expectLater(
        () => repo.init(
          GitIntegrationProvider.gitlab,
          redirectUri: 'https://example.app/cb',
          host: 'http://127.0.0.1',
          byoClientId: 'cid',
          byoClientSecret: 'secret',
        ),
        throwsA(
          isA<GitIntegrationsException>()
              .having((e) => e.errorCode, 'errorCode', 'invalid_host')
              .having((e) => e.statusCode, 'statusCode', 400),
        ),
      );
    });

    test('completeCallback — 410 user_cancelled → exception', () async {
      when(
        () => dio.post<Map<String, dynamic>>(
          '/integrations/github/auth/callback',
          data: any(named: 'data'),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenThrow(
        DioException(
          requestOptions: RequestOptions(
            path: '/integrations/github/auth/callback',
          ),
          response: Response(
            data: const <String, dynamic>{
              'error_code': 'user_cancelled',
              'message': 'User cancelled authorization',
            },
            statusCode: 410,
            requestOptions: RequestOptions(
              path: '/integrations/github/auth/callback',
            ),
          ),
          type: DioExceptionType.badResponse,
        ),
      );

      await expectLater(
        () => repo.completeCallback(
          GitIntegrationProvider.github,
          code: 'c',
          state: 'st',
          error: 'access_denied',
        ),
        throwsA(
          isA<GitIntegrationsException>()
              .having((e) => e.errorCode, 'errorCode', 'user_cancelled')
              .having((e) => e.statusCode, 'statusCode', 410),
        ),
      );
    });

    test('revoke — returns remote_revoke_failed=true from body', () async {
      when(
        () => dio.delete<Map<String, dynamic>>(
          '/integrations/github/auth/revoke',
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{
            'provider': 'github',
            'remote_revoke_failed': true,
          },
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/github/auth/revoke',
          ),
        ),
      );

      final remoteFailed = await repo.revoke(GitIntegrationProvider.github);
      expect(remoteFailed, isTrue);
    });

    test('revoke — defaults to false when field absent', () async {
      when(
        () => dio.delete<Map<String, dynamic>>(
          '/integrations/gitlab/auth/revoke',
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<Map<String, dynamic>>(
          data: const <String, dynamic>{'provider': 'gitlab'},
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/integrations/gitlab/auth/revoke',
          ),
        ),
      );

      final remoteFailed = await repo.revoke(GitIntegrationProvider.gitlab);
      expect(remoteFailed, isFalse);
    });
  });
}
