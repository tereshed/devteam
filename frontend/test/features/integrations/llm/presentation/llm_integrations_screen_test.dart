// @Tags(['widget'])
//
// Widget-тесты `LlmIntegrationsScreen` — три акцент-state'а: loading,
// connected, empty (AC 2.6 из docs/tasks/ui_refactoring/tasks-breakdown.md).

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/claude_code_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/integrations/llm/presentation/screens/llm_integrations_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';

class _FakeRepo implements LlmIntegrationsRepository {
  _FakeRepo({
    this.apiKey = const <LlmProviderConnection>[],
    this.claude =
        const ClaudeCodeIntegrationStatus(connected: false),
    this.gate,
  });

  List<LlmProviderConnection> apiKey;
  ClaudeCodeIntegrationStatus claude;
  Completer<void>? gate;

  @override
  Future<List<LlmProviderConnection>> fetchApiKeyConnections({
    cancelToken,
  }) async {
    if (gate != null) {
      await gate!.future;
    }
    return apiKey;
  }

  @override
  Future<ClaudeCodeIntegrationStatus> fetchClaudeCodeStatus({cancelToken}) async {
    return claude;
  }

  @override
  noSuchMethod(Invocation invocation) {
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
      // dioClientProvider used by repositoryProvider — fake repo bypasses it.
      dioClientProvider.overrideWithValue(Dio()),
      llmIntegrationsRepositoryProvider.overrideWithValue(repo),
      webSocketServiceProvider.overrideWithValue(ws),
    ],
  );
}

Future<void> _pump(
  WidgetTester tester,
  ProviderContainer container,
) async {
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: LlmIntegrationsScreen()),
      ),
    ),
  );
}

void main() {
  group('LlmIntegrationsScreen', () {
    testWidgets('loading: показывает CircularProgressIndicator до первого refresh',
        (tester) async {
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
      // postFrameCallback запускает refresh(); gate не завершен → state.isLoading == true.
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      // Завершаем gate, чтобы убрать pending-Timer перед disposal.
      gate.complete();
      await tester.pumpAndSettle();
    });

    testWidgets('connected: показывает карточку Claude Code в секции Connected',
        (tester) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final repo = _FakeRepo(
        apiKey: const [
          LlmProviderConnection(
            provider: LlmIntegrationProvider.anthropic,
            status: LlmProviderConnectionStatus.connected,
            maskedPreview: '****3Mk9',
          ),
        ],
        claude:
            const ClaudeCodeIntegrationStatus(connected: true),
      );
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pumpAndSettle();

      expect(find.text('Connected'), findsWidgets);
      expect(find.text('Claude Code'), findsOneWidget);
      expect(find.text('Anthropic'), findsOneWidget);
      // masked_preview виден в карточке как statusDetail
      expect(find.text('****3Mk9'), findsOneWidget);
    });

    testWidgets('empty: connected секция показывает empty hint, available — все 6 карточек',
        (tester) async {
      tester.view.physicalSize = const Size(1400, 900);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);

      final repo = _FakeRepo(); // ничего не подключено
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);

      await _pump(tester, container);
      await tester.pumpAndSettle();

      // Connected секция пустая → видим empty hint.
      expect(find.text('All supported providers are already connected.'),
          findsOneWidget);
      // Available секция содержит все 6 заявленных провайдеров.
      expect(find.text('Claude Code'), findsOneWidget);
      expect(find.text('Anthropic'), findsOneWidget);
      expect(find.text('OpenAI'), findsOneWidget);
      expect(find.text('OpenRouter'), findsOneWidget);
      expect(find.text('DeepSeek'), findsOneWidget);
      expect(find.text('Zhipu'), findsOneWidget);
      // Кнопка "Connect" есть как минимум один раз.
      expect(find.text('Connect'), findsWidgets);
    });
  });
}
