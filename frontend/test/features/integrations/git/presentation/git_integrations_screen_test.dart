// @Tags(['widget'])
//
// Widget-тесты `GitIntegrationsScreen` — три state'а: loading, connected,
// empty. AC: 3.11 (широкоформатный/мобильный layout), 3.13 (acceptance Этапа 3b).

import 'dart:async';

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
import 'package:frontend/features/integrations/git/presentation/screens/git_integrations_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';

class _FakeRepo implements GitIntegrationsRepository {
  _FakeRepo({this.snapshot = const {}, this.gate});

  Map<GitIntegrationProvider, GitProviderConnection> snapshot;
  Completer<void>? gate;

  @override
  Future<GitProviderConnection> fetchStatus(
    GitIntegrationProvider provider, {
    cancelToken,
  }) async {
    if (gate != null) {
      await gate!.future;
    }
    return snapshot[provider] ??
        GitProviderConnection(
          provider: provider,
          status: GitProviderConnectionStatus.disconnected,
        );
  }

  @override
  dynamic noSuchMethod(Invocation invocation) {
    throw UnimplementedError('${invocation.memberName}');
  }
}

class _FakeWebSocketService extends WebSocketService {
  _FakeWebSocketService()
    : super(
        baseUrl: 'http://127.0.0.1:8080/api/v1',
        channelFactory: (_, {protocols}) =>
            throw UnimplementedError('not used in tests'),
        authProvider: () async => const WsAuth.none(),
      );

  final _ctrl = StreamController<WsClientEvent>.broadcast();

  @override
  Stream<WsClientEvent> get events => _ctrl.stream;

  void close() => _ctrl.close();
}

ProviderContainer _container({
  required _FakeRepo repo,
  required _FakeWebSocketService ws,
}) {
  return ProviderContainer(
    overrides: [
      dioClientProvider.overrideWithValue(Dio()),
      gitIntegrationsRepositoryProvider.overrideWithValue(repo),
      webSocketServiceProvider.overrideWithValue(ws),
    ],
  );
}

Future<void> _pump(WidgetTester tester, ProviderContainer container) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: const MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: GitIntegrationsScreen()),
      ),
    ),
  );
}

void main() {
  group('GitIntegrationsScreen', () {
    testWidgets('loading: CircularProgressIndicator до первого refresh', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final gate = Completer<void>();
      final repo = _FakeRepo(gate: gate);
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      gate.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('connected: GitHub в секции Connected, GitLab — Available', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final repo = _FakeRepo(
        snapshot: const {
          GitIntegrationProvider.github: GitProviderConnection(
            provider: GitIntegrationProvider.github,
            status: GitProviderConnectionStatus.connected,
            accountLogin: 'octocat',
          ),
        },
      );
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pumpAndSettle();

      expect(find.text('GitHub'), findsOneWidget);
      expect(find.text('GitLab'), findsOneWidget);
      // Connected chip + account_login subtitle.
      expect(find.text('Connected'), findsWidgets);
      expect(find.text('Account: octocat'), findsOneWidget);
      // Disconnect-кнопка у GitHub, Connect у GitLab.
      expect(find.text('Disconnect'), findsOneWidget);
      expect(find.text('Connect'), findsWidgets);
      // Self-hosted CTA только у GitLab.
      expect(find.text('Connect self-hosted'), findsOneWidget);
    });

    testWidgets('empty: оба disconnected → empty hint в Connected', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final repo = _FakeRepo();
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pumpAndSettle();

      // Empty hint в Connected.
      expect(
        find.text('All supported providers are already connected.'),
        findsOneWidget,
      );
      // Оба провайдера в Available.
      expect(find.text('GitHub'), findsOneWidget);
      expect(find.text('GitLab'), findsOneWidget);
    });

    testWidgets('mobile layout (<600): single column', (tester) async {
      tester.view.physicalSize = const Size(420, 800);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final repo = _FakeRepo();
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pumpAndSettle();

      // На мобильном — карточки тоже видны, screen не падает.
      expect(find.text('GitHub'), findsOneWidget);
      expect(find.text('GitLab'), findsOneWidget);
    });
  });
}
