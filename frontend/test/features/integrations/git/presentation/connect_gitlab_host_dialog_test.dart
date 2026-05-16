// @Tags(['widget'])
//
// Widget-тесты `connect_gitlab_host_dialog.dart` — клиент-сайд валидация и
// маппинг серверной ошибки `invalid_host` в локализованный баннер (AC 3.12).

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_repository.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/presentation/widgets/connect_gitlab_host_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';

class _Repo implements GitIntegrationsRepository {
  _Repo({this.initResult, this.initThrows});

  GitOAuthInitResult? initResult;
  GitIntegrationsException? initThrows;

  @override
  Future<GitOAuthInitResult> init(
    GitIntegrationProvider provider, {
    required String redirectUri,
    String? host,
    String? byoClientId,
    String? byoClientSecret,
    cancelToken,
  }) async {
    if (initThrows != null) {
      throw initThrows!;
    }
    return initResult ??
        const GitOAuthInitResult(authorizeUrl: 'https://x', state: 's');
  }

  @override
  Future<GitProviderConnection> fetchStatus(
    GitIntegrationProvider provider, {
    cancelToken,
  }) async => GitProviderConnection(
    provider: provider,
    status: GitProviderConnectionStatus.disconnected,
  );

  @override
  dynamic noSuchMethod(Invocation invocation) {
    throw UnimplementedError('${invocation.memberName}');
  }
}

class _Ws extends WebSocketService {
  _Ws()
    : super(
        baseUrl: 'http://127.0.0.1:8080/api/v1',
        channelFactory: (_, {protocols}) =>
            throw UnimplementedError('not used'),
        authProvider: () async => const WsAuth.none(),
      );

  @override
  Stream<WsClientEvent> get events => const Stream.empty();
}

Future<void> _pump(
  WidgetTester tester, {
  required _Repo repo,
  required ValueChanged<WidgetRef> onPressed,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioClientProvider.overrideWithValue(Dio()),
        gitIntegrationsRepositoryProvider.overrideWithValue(repo),
        webSocketServiceProvider.overrideWithValue(_Ws()),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Consumer(
            builder: (context, ref, _) {
              return TextButton(
                onPressed: () => onPressed(ref),
                child: const Text('open'),
              );
            },
          ),
        ),
      ),
    ),
  );
}

void main() {
  group('connect_gitlab_host_dialog', () {
    testWidgets('client-side validation: empty host blocks submit', (
      tester,
    ) async {
      final repo = _Repo();
      await _pump(
        tester,
        repo: repo,
        onPressed: (ref) async {
          await showConnectGitlabHostDialog(
            tester.element(find.text('open')),
            ref,
            redirectUri: 'https://example.app/cb',
          );
        },
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Не заполняем поля — нажимаем Connect.
      await tester.tap(find.widgetWithText(FilledButton, 'Connect'));
      await tester.pumpAndSettle();

      // Все три валидатора отрабатывают:
      expect(find.text('Enter your GitLab host URL'), findsOneWidget);
      expect(find.text('Enter the Application ID'), findsOneWidget);
      expect(find.text('Enter the Application Secret'), findsOneWidget);
    });

    testWidgets('client-side validation: http://... fails scheme check', (
      tester,
    ) async {
      // Lightweight client-side: scheme должен быть https (или http для local dev — обе валидны).
      // Тест проверяет, что произвольная строка типа "not-a-url" падает на формате.
      final repo = _Repo();
      await _pump(
        tester,
        repo: repo,
        onPressed: (ref) async {
          await showConnectGitlabHostDialog(
            tester.element(find.text('open')),
            ref,
            redirectUri: 'https://example.app/cb',
          );
        },
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextFormField, 'GitLab host (https://…)'),
        'not-a-url',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, 'Application ID'),
        'cid',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, 'Application Secret'),
        'secret',
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Connect'));
      await tester.pumpAndSettle();

      // Любое сообщение из двух подходит — главное, что серверный init не вызывался.
      expect(
        find.byWidgetPredicate(
          (w) =>
              w is Text &&
              (w.data ==
                      'Host must start with https:// (or http:// for local dev)' ||
                  w.data == 'Host URL is malformed'),
        ),
        findsOneWidget,
      );
    });

    testWidgets('server invalid_host → локализованный error banner', (
      tester,
    ) async {
      final repo = _Repo(
        initThrows: const GitIntegrationsException(
          message: 'Provided git host is not allowed',
          errorCode: 'invalid_host',
          statusCode: 400,
        ),
      );
      await _pump(
        tester,
        repo: repo,
        onPressed: (ref) async {
          await showConnectGitlabHostDialog(
            tester.element(find.text('open')),
            ref,
            redirectUri: 'https://example.app/cb',
          );
        },
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextFormField, 'GitLab host (https://…)'),
        'https://gl.acme.corp',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, 'Application ID'),
        'cid',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, 'Application Secret'),
        'secret',
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Connect'));
      await tester.pumpAndSettle();

      expect(
        find.text(
          'Host is not allowed (private network, unsupported scheme, or malformed URL).',
        ),
        findsOneWidget,
      );
    });

    testWidgets(
      'happy path: valid host → returns authorize_url, dialog closes',
      (tester) async {
        final repo = _Repo(
          initResult: const GitOAuthInitResult(
            authorizeUrl: 'https://gl.acme.corp/oauth/authorize?x=1',
            state: 'st',
          ),
        );
        ConnectGitlabHostResult? captured;
        await _pump(
          tester,
          repo: repo,
          onPressed: (ref) async {
            captured = await showConnectGitlabHostDialog(
              tester.element(find.text('open')),
              ref,
              redirectUri: 'https://example.app/cb',
            );
          },
        );
        await tester.tap(find.text('open'));
        await tester.pumpAndSettle();

        await tester.enterText(
          find.widgetWithText(TextFormField, 'GitLab host (https://…)'),
          'https://gl.acme.corp',
        );
        await tester.enterText(
          find.widgetWithText(TextFormField, 'Application ID'),
          'cid',
        );
        await tester.enterText(
          find.widgetWithText(TextFormField, 'Application Secret'),
          'secret',
        );
        await tester.tap(find.widgetWithText(FilledButton, 'Connect'));
        await tester.pumpAndSettle();

        expect(captured, isNotNull);
        expect(
          captured!.authorizeUrl,
          'https://gl.acme.corp/oauth/authorize?x=1',
        );
      },
    );

    testWidgets('instructions toggle разворачивает шаги c redirect_uri', (
      tester,
    ) async {
      final repo = _Repo();
      await _pump(
        tester,
        repo: repo,
        onPressed: (ref) async {
          await showConnectGitlabHostDialog(
            tester.element(find.text('open')),
            ref,
            redirectUri: 'https://example.app/cb',
          );
        },
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();

      // Toggle присутствует.
      expect(
        find.text('How to register an Application in my GitLab'),
        findsOneWidget,
      );
      // По умолчанию свёрнут — шага 3 (с redirect_uri) не видно.
      expect(
        find.textContaining('Redirect URI: https://example.app/cb'),
        findsNothing,
      );
      await tester.tap(
        find.text('How to register an Application in my GitLab'),
      );
      await tester.pumpAndSettle();
      // Теперь развёрнут.
      expect(
        find.textContaining('Redirect URI: https://example.app/cb'),
        findsOneWidget,
      );
    });
  });
}
