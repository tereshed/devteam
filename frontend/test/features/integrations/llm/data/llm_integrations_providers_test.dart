// @dart=2.19
// @TestOn('vm')
// @Tags(['unit'])

import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/claude_code_status_model.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

class _FakeRepo implements LlmIntegrationsRepository {
  _FakeRepo({
    this.apiKeyConnections = const <LlmProviderConnection>[],
    this.claudeStatus =
        const ClaudeCodeIntegrationStatus(connected: false),
  });

  List<LlmProviderConnection> apiKeyConnections;
  ClaudeCodeIntegrationStatus claudeStatus;
  int statusCallCount = 0;

  @override
  Future<List<LlmProviderConnection>> fetchApiKeyConnections({
    cancelToken,
  }) async {
    return apiKeyConnections;
  }

  @override
  Future<ClaudeCodeIntegrationStatus> fetchClaudeCodeStatus({
    cancelToken,
  }) async {
    statusCallCount++;
    return claudeStatus;
  }

  @override
  noSuchMethod(Invocation invocation) {
    throw UnimplementedError(
      'Метод ${invocation.memberName} не реализован в _FakeRepo',
    );
  }
}

void main() {
  group('LlmIntegrationsController', () {
    test('refresh() — собирает API-key и Claude Code в один Map', () async {
      final repo = _FakeRepo(
        apiKeyConnections: [
          LlmProviderConnection(
            provider: LlmIntegrationProvider.anthropic,
            status: LlmProviderConnectionStatus.connected,
            maskedPreview: '****abcd',
          ),
          LlmProviderConnection(
            provider: LlmIntegrationProvider.openai,
            status: LlmProviderConnectionStatus.disconnected,
          ),
        ],
        claudeStatus: ClaudeCodeIntegrationStatus(
          connected: true,
          expiresAt: DateTime.utc(2030, 1, 1),
        ),
      );
      final ws = StreamController<WsClientEvent>.broadcast();
      addTearDown(ws.close);

      final c = LlmIntegrationsController(
        repository: repo,
        wsEvents: ws.stream,
      );
      addTearDown(c.dispose);

      await c.refresh();

      final s = c.state;
      expect(s.isLoading, isFalse);
      expect(s.errorMessage, isNull);
      expect(
        s.connections[LlmIntegrationProvider.anthropic]?.status,
        LlmProviderConnectionStatus.connected,
      );
      expect(
        s.connections[LlmIntegrationProvider.claudeCodeOAuth]?.status,
        LlmProviderConnectionStatus.connected,
      );
      expect(
        s.connections[LlmIntegrationProvider.openai]?.status,
        LlmProviderConnectionStatus.disconnected,
      );
    });

    test('подписка на IntegrationConnectionChanged мутирует state', () async {
      final repo = _FakeRepo();
      final ws = StreamController<WsClientEvent>.broadcast();
      addTearDown(ws.close);

      final c = LlmIntegrationsController(
        repository: repo,
        wsEvents: ws.stream,
      );
      addTearDown(c.dispose);

      // Не делаем refresh: проверяем чисто WS-путь.
      ws.add(WsClientEvent.server(
        WsServerEvent.integrationStatus(
          WsIntegrationStatusEvent(
            ts: DateTime.utc(2030, 1, 1),
            v: 1,
            userId: 'user-1',
            provider: 'claude_code_oauth',
            status: WsIntegrationStatus.connected,
          ),
        ),
      ));

      await Future<void>.delayed(Duration.zero);

      expect(
        c.state.connections[LlmIntegrationProvider.claudeCodeOAuth]?.status,
        LlmProviderConnectionStatus.connected,
      );
    });

    test('reconnect (transient failure → server event) запускает повторный GET /status',
        () async {
      final repo = _FakeRepo();
      final ws = StreamController<WsClientEvent>.broadcast();
      addTearDown(ws.close);

      final c = LlmIntegrationsController(
        repository: repo,
        wsEvents: ws.stream,
      );
      addTearDown(c.dispose);

      await c.refresh();
      final firstCallCount = repo.statusCallCount;

      // Сначала транзитный сбой WS — controller помечает «надо сделать resync».
      ws.add(const WsClientEvent.serviceFailure(
        WsServiceFailure.transient(),
      ));
      await Future<void>.delayed(Duration.zero);
      expect(repo.statusCallCount, firstCallCount,
          reason: 'на сбое НЕ должен сразу пере-фетчить (сокет ещё в reconnect)');

      // Теперь приходит обычный server-event — это сигнал «сокет снова жив».
      ws.add(WsClientEvent.server(
        WsServerEvent.integrationStatus(
          WsIntegrationStatusEvent(
            ts: DateTime.utc(2030, 1, 1),
            v: 1,
            userId: 'user-1',
            provider: 'claude_code_oauth',
            status: WsIntegrationStatus.pending,
          ),
        ),
      ));
      // Даём микротакам отработать.
      await Future<void>.delayed(const Duration(milliseconds: 5));

      expect(repo.statusCallCount, greaterThan(firstCallCount),
          reason: 'после reconnect должен быть второй GET /status');
    });

    test('parse/auth-event без server-event resync не вызывает', () async {
      final repo = _FakeRepo();
      final ws = StreamController<WsClientEvent>.broadcast();
      addTearDown(ws.close);

      final c = LlmIntegrationsController(
        repository: repo,
        wsEvents: ws.stream,
      );
      addTearDown(c.dispose);

      await c.refresh();
      final before = repo.statusCallCount;

      ws.add(const WsClientEvent.parseError(
        WsParseError(message: 'x'),
      ));
      await Future<void>.delayed(Duration.zero);

      expect(repo.statusCallCount, before);
    });
  });
}
