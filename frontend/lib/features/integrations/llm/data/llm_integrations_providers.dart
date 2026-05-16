import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

/// DI: Singleton-репозиторий LLM Integrations.
final llmIntegrationsRepositoryProvider = Provider<LlmIntegrationsRepository>((
  ref,
) {
  final dio = ref.watch(dioClientProvider);
  return LlmIntegrationsRepository(dio: dio);
});

/// Снимок состояния экрана LLM Integrations.
@immutable
class LlmIntegrationsState {
  const LlmIntegrationsState({
    required this.connections,
    this.isLoading = false,
    this.errorMessage,
  });

  static const LlmIntegrationsState initial = LlmIntegrationsState(
    connections: <LlmIntegrationProvider, LlmProviderConnection>{},
    isLoading: true,
  );

  final Map<LlmIntegrationProvider, LlmProviderConnection> connections;
  final bool isLoading;
  final String? errorMessage;

  LlmIntegrationsState copyWith({
    Map<LlmIntegrationProvider, LlmProviderConnection>? connections,
    bool? isLoading,
    Object? errorMessage = _noChange,
  }) {
    return LlmIntegrationsState(
      connections: connections ?? this.connections,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: identical(errorMessage, _noChange)
          ? this.errorMessage
          : errorMessage as String?,
    );
  }

  static const _noChange = Object();
}

/// Контроллер экрана LLM Integrations: первичный REST-fetch + подписка на WS.
///
/// Поведение (UI Refactoring §4a.4):
///   1. `refresh()` — `GET /me/llm-credentials` + `GET /claude-code/auth/status`.
///   2. Подписка на [WsServerEventIntegrationStatus] — обновление без REST.
///   3. После WS service-failure следующий server-event считается «reconnect» —
///      запускает повторный `refresh()`, чтобы покрыть пропуски.
///
/// Реализация — [ChangeNotifier]. Riverpod 3.x не имеет `ChangeNotifierProvider`,
/// поэтому стейт экспонируется как [Stream] через [llmIntegrationsStateProvider]
/// (`StreamProvider` поверх `addListener`); сам контроллер раздаётся через
/// обычный `Provider` для императивных вызовов (`refresh()`, `applyLocal()`).
class LlmIntegrationsController extends ChangeNotifier {
  LlmIntegrationsController({
    required LlmIntegrationsRepository repository,
    required Stream<WsClientEvent> wsEvents,
  }) : _repository = repository,
       _state = LlmIntegrationsState.initial {
    _wsSubscription = wsEvents.listen(_onWsClientEvent);
  }

  final LlmIntegrationsRepository _repository;
  StreamSubscription<WsClientEvent>? _wsSubscription;
  LlmIntegrationsState _state;
  bool _needsResyncOnNextServerEvent = false;
  bool _disposed = false;

  /// Монотонно растущая «версия» стейта. Инкрементируется при старте `refresh()`
  /// и при каждой мутации из WS (`applyLocal`). Если за время летящего REST-запроса
  /// номер вырастет, мы знаем, что в стейт уже попали более свежие данные из WS, и
  /// результат REST не должен их перезатереть. Решает race из stage 2 ревью §1.
  int _stateVersion = 0;

  LlmIntegrationsState get state => _state;

  void _setState(LlmIntegrationsState next) {
    if (_disposed) {
      return;
    }
    _state = next;
    notifyListeners();
  }

  /// Полный REST-rebuild. Вызывается при первом open экрана и при reconnect WS.
  ///
  /// Race-protection: фиксируем `_stateVersion` на входе. Если за время REST'а
  /// прилетит WS-событие или другой `refresh()` — оно увеличит счётчик, и мы
  /// тихо выходим, не перезаписывая свежий стейт устаревшим снимком из БД-реплики.
  Future<void> refresh() async {
    final startedAtVersion = ++_stateVersion;
    _setState(_state.copyWith(isLoading: _state.connections.isEmpty));
    try {
      final apiKeyConnections = await _repository.fetchApiKeyConnections();
      final claudeStatus = await _repository.fetchClaudeCodeStatus();
      if (_disposed || startedAtVersion != _stateVersion) {
        return;
      }

      final connections = <LlmIntegrationProvider, LlmProviderConnection>{};
      for (final c in apiKeyConnections) {
        connections[c.provider] = c;
      }
      connections[LlmIntegrationProvider.claudeCodeOAuth] =
          LlmProviderConnection(
            provider: LlmIntegrationProvider.claudeCodeOAuth,
            status: claudeStatus.connected
                ? LlmProviderConnectionStatus.connected
                : LlmProviderConnectionStatus.disconnected,
            expiresAt: claudeStatus.expiresAt,
          );
      _setState(
        _state.copyWith(
          connections: connections,
          isLoading: false,
          errorMessage: null,
        ),
      );
    } catch (e) {
      if (_disposed || startedAtVersion != _stateVersion) {
        return;
      }
      _setState(_state.copyWith(isLoading: false, errorMessage: e.toString()));
    }
  }

  /// Локальная мутация без сети — для диалогов, инициирующих flow, и для
  /// применения событий из WS. Инкремент `_stateVersion` инвалидирует любой
  /// летящий `refresh()`, чтобы он не затёр свежий статус устаревшим снимком.
  void applyLocal(LlmProviderConnection connection) {
    _stateVersion++;
    final next = Map<LlmIntegrationProvider, LlmProviderConnection>.from(
      _state.connections,
    )..[connection.provider] = connection;
    _setState(_state.copyWith(connections: next));
  }

  @visibleForTesting
  bool get debugNeedsResyncOnNextServerEvent => _needsResyncOnNextServerEvent;

  void _onWsClientEvent(WsClientEvent ev) {
    switch (ev) {
      case WsClientEventServiceFailure():
      case WsClientEventAuthFailure():
        _needsResyncOnNextServerEvent = true;
        return;
      case WsClientEventSubprotocolMismatch():
      case WsClientEventParseError():
        return;
      case WsClientEventServer(:final event):
        // Сначала применяем сам ивент (синхронно, без сети) — это инкрементит
        // _stateVersion. Потом запускаем resync: даже если он вернёт устаревший
        // снимок из БД-реплики, version-guard в refresh() отбросит его.
        event.when(
          taskStatus: (_) {},
          taskMessage: (_) {},
          agentLog: (_) {},
          error: (_) {},
          integrationStatus: _applyIntegrationStatus,
          unknown: (_) {},
        );
        if (_needsResyncOnNextServerEvent) {
          _needsResyncOnNextServerEvent = false;
          unawaited(refresh());
        }
        return;
    }
  }

  void _applyIntegrationStatus(WsIntegrationStatusEvent event) {
    final provider = LlmIntegrationProvider.tryParse(event.provider);
    if (provider == null) {
      return;
    }
    final current = _state.connections[provider];
    final next = LlmProviderConnection(
      provider: provider,
      status: _toDomainStatus(event.status),
      maskedPreview: current?.maskedPreview,
      reason: event.reason,
      connectedAt: event.connectedAt,
      expiresAt: event.expiresAt,
    );
    applyLocal(next);
  }

  static LlmProviderConnectionStatus _toDomainStatus(WsIntegrationStatus s) {
    switch (s) {
      case WsIntegrationStatus.connected:
        return LlmProviderConnectionStatus.connected;
      case WsIntegrationStatus.disconnected:
        return LlmProviderConnectionStatus.disconnected;
      case WsIntegrationStatus.error:
        return LlmProviderConnectionStatus.error;
      case WsIntegrationStatus.pending:
        return LlmProviderConnectionStatus.pending;
    }
  }

  @override
  void dispose() {
    _disposed = true;
    unawaited(_wsSubscription?.cancel());
    _wsSubscription = null;
    super.dispose();
  }
}

/// Live-провайдер контроллера экрана LLM Integrations.
///
/// Возвращает long-lived контроллер; UI листает `state` через
/// [llmIntegrationsStateProvider].
final llmIntegrationsControllerProvider = Provider<LlmIntegrationsController>((
  ref,
) {
  final repo = ref.watch(llmIntegrationsRepositoryProvider);
  final ws = ref.watch(webSocketServiceProvider);
  final controller = LlmIntegrationsController(
    repository: repo,
    wsEvents: ws.events,
  );
  ref.onDispose(controller.dispose);
  return controller;
});

/// Стрим-провайдер state для виджетов: реагирует на каждый `notifyListeners`.
///
/// Использует [Stream] — Riverpod 3 не имеет `ChangeNotifierProvider`,
/// но позволяет легко превращать [Listenable] в [Stream] через `addListener`.
final llmIntegrationsStateProvider = StreamProvider<LlmIntegrationsState>((
  ref,
) {
  final controller = ref.watch(llmIntegrationsControllerProvider);
  final controller$ = StreamController<LlmIntegrationsState>();
  void onChange() => controller$.add(controller.state);
  controller.addListener(onChange);
  // первое значение — для синхронного first build.
  scheduleMicrotask(onChange);
  ref.onDispose(() {
    controller.removeListener(onChange);
    unawaited(controller$.close());
  });
  return controller$.stream;
});
