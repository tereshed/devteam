import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_repository.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';

/// DI: Singleton-репозиторий Git Integrations.
final gitIntegrationsRepositoryProvider = Provider<GitIntegrationsRepository>((
  ref,
) {
  final dio = ref.watch(dioClientProvider);
  return GitIntegrationsRepository(dio: dio);
});

/// Снимок состояния экрана Git Integrations.
@immutable
class GitIntegrationsState {
  const GitIntegrationsState({
    required this.connections,
    this.isLoading = false,
    this.errorMessage,
  });

  static const GitIntegrationsState initial = GitIntegrationsState(
    connections: <GitIntegrationProvider, GitProviderConnection>{},
    isLoading: true,
  );

  final Map<GitIntegrationProvider, GitProviderConnection> connections;
  final bool isLoading;
  final String? errorMessage;

  GitIntegrationsState copyWith({
    Map<GitIntegrationProvider, GitProviderConnection>? connections,
    bool? isLoading,
    Object? errorMessage = _noChange,
  }) {
    return GitIntegrationsState(
      connections: connections ?? this.connections,
      isLoading: isLoading ?? this.isLoading,
      errorMessage: identical(errorMessage, _noChange)
          ? this.errorMessage
          : errorMessage as String?,
    );
  }

  static const _noChange = Object();
}

/// Контроллер экрана Git Integrations: первичный REST-fetch + подписка на WS.
///
/// Поведение (зеркало 2.5/LLM-контроллера, UI Refactoring §4a.4):
///   1. `refresh()` — `GET /status` для обоих провайдеров.
///   2. Подписка на [WsServerEventIntegrationStatus] — обновление без REST.
///   3. После WS service-failure следующий server-event считается «reconnect» —
///      запускает повторный `refresh()`, чтобы покрыть пропуски во время обрыва.
///
/// Race-protection как в LLM-контроллере: монотонный `_stateVersion`. Если за время
/// летящего REST-запроса прилетит WS-событие или ещё один `refresh()` — счётчик
/// инкрементится и устаревший REST-снимок не перетрёт свежий стейт.
class GitIntegrationsController extends Notifier<GitIntegrationsState> {
  StreamSubscription<WsClientEvent>? _wsSubscription;
  bool _needsResyncOnNextServerEvent = false;
  int _stateVersion = 0;

  @override
  GitIntegrationsState build() {
    final ws = ref.watch(webSocketServiceProvider);
    _wsSubscription = ws.events.listen(_onWsClientEvent);
    ref.onDispose(() {
      unawaited(_wsSubscription?.cancel());
    });
    // Запускаем первичный fetch
    scheduleMicrotask(refresh);
    return GitIntegrationsState.initial;
  }

  /// Полный REST-rebuild: `GET /status` для обоих провайдеров.
  /// Вызывается при первом open экрана и при reconnect WS.
  Future<void> refresh() async {
    final startedAtVersion = ++_stateVersion;
    state = state.copyWith(isLoading: state.connections.isEmpty);
    try {
      final repository = ref.read(gitIntegrationsRepositoryProvider);
      final results = await Future.wait(
        GitIntegrationProvider.values.map(repository.fetchStatus),
      );
      if (startedAtVersion != _stateVersion) {
        return;
      }
      final connections = <GitIntegrationProvider, GitProviderConnection>{};
      for (final c in results) {
        connections[c.provider] = c;
      }
      state = state.copyWith(
        connections: connections,
        isLoading: false,
        errorMessage: null,
      );
    } catch (e) {
      if (startedAtVersion != _stateVersion) {
        return;
      }
      state = state.copyWith(isLoading: false, errorMessage: e.toString());
    }
  }

  /// Локальная мутация без сети — для применения событий из WS и внутренних
  /// flow-методов контроллера. Инкремент `_stateVersion` инвалидирует любой
  /// летящий `refresh()`.
  void applyLocal(GitProviderConnection connection) {
    _stateVersion++;
    final next = Map<GitIntegrationProvider, GitProviderConnection>.from(
      state.connections,
    )..[connection.provider] = connection;
    state = state.copyWith(connections: next);
  }

  /// Стартует OAuth flow: сразу выставляет локальный `pending` (чтобы кнопка
  /// заблокировалась и показала лоадер пока летит сетевой запрос), затем дёргает
  /// `POST /init`, возвращает `authorize_url`. UI должен сам открыть URL в
  /// браузере, и если открыть не удалось — позвать [rollbackToDisconnected].
  ///
  /// Race-protection: если за время `await` стейт уехал из `pending` (например,
  /// UI/диалог откатил его при отмене), бросаем `cancelled_locally` вместо
  /// возврата URL — иначе карточка ушла бы в pending после отмены.
  ///
  /// При [GitIntegrationsException] обновляет state в `error(reason=errorCode)`,
  /// при любой другой ошибке — `error(reason='internal_error')`, и пробрасывает
  /// исключение наверх. [updateStateOnError]=false означает «BYO-диалог сам
  /// управляет своим error UI» — тогда катим стейт обратно в `disconnected`,
  /// чтобы карточка GitLab не светила error/pending в фоне за диалогом.
  Future<String> initConnection(
    GitIntegrationProvider provider, {
    required String redirectUri,
    String? host,
    String? byoClientId,
    String? byoClientSecret,
    bool updateStateOnError = true,
  }) async {
    applyLocal(
      GitProviderConnection(
        provider: provider,
        status: GitProviderConnectionStatus.pending,
        host: host,
      ),
    );
    try {
      final out = await ref
          .read(gitIntegrationsRepositoryProvider)
          .init(
            provider,
            redirectUri: redirectUri,
            host: host,
            byoClientId: byoClientId,
            byoClientSecret: byoClientSecret,
          );
      // Race: пока летел init, кто-то откатил pending (например, диалог
      // закрыли). Не возвращаем URL — иначе UI откроет браузер впустую и
      // карточка зависнет.
      if (state.connections[provider]?.status !=
          GitProviderConnectionStatus.pending) {
        throw const GitIntegrationsException(message: 'cancelled_locally');
      }
      return out.authorizeUrl;
    } on GitIntegrationsException catch (e) {
      if (updateStateOnError) {
        applyLocal(
          GitProviderConnection(
            provider: provider,
            status: GitProviderConnectionStatus.error,
            reason: e.errorCode,
          ),
        );
      } else {
        rollbackToDisconnected(provider, host: host);
      }
      rethrow;
    } catch (_) {
      // SocketException / TypeError / etc — карточка не должна остаться в pending.
      if (updateStateOnError) {
        applyLocal(
          GitProviderConnection(
            provider: provider,
            status: GitProviderConnectionStatus.error,
            reason: 'internal_error',
          ),
        );
      } else {
        rollbackToDisconnected(provider, host: host);
      }
      rethrow;
    }
  }

  /// Отзыв подключения через `DELETE /revoke`. На сетевой ошибке делает
  /// background `refresh()`, чтобы UI догнал реальный стейт сервера.
  Future<void> disconnect(GitIntegrationProvider provider) async {
    try {
      final remoteFailed = await ref
          .read(gitIntegrationsRepositoryProvider)
          .revoke(provider);
      applyLocal(
        GitProviderConnection(
          provider: provider,
          status: GitProviderConnectionStatus.disconnected,
          remoteRevokeFailed: remoteFailed,
        ),
      );
    } catch (_) {
      unawaited(refresh());
    }
  }

  /// Откат локального `pending` обратно в `disconnected` — UI вызывает, если
  /// не удалось открыть `authorize_url` в браузере (`launchUrl` вернул `false`).
  /// Без этого кнопка «Connect» осталась бы навсегда `isBusy`.
  void rollbackToDisconnected(GitIntegrationProvider provider, {String? host}) {
    applyLocal(
      GitProviderConnection(
        provider: provider,
        status: GitProviderConnectionStatus.disconnected,
        host: host,
      ),
    );
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
    final provider = GitIntegrationProvider.tryParse(event.provider);
    if (provider == null) {
      // Не наш провайдер (например, claude_code_oauth) — игнорируем.
      return;
    }
    final current = state.connections[provider];
    final next = GitProviderConnection(
      provider: provider,
      status: _toDomainStatus(event.status),
      // host/accountLogin/scopes из WS не приходят — оставляем последние известные;
      // следующий `refresh()` подтянет актуальные.
      host: current?.host,
      accountLogin: current?.accountLogin,
      scopes: current?.scopes,
      reason: event.reason,
      connectedAt: event.connectedAt ?? current?.connectedAt,
      expiresAt: event.expiresAt ?? current?.expiresAt,
    );
    applyLocal(next);
  }

  static GitProviderConnectionStatus _toDomainStatus(WsIntegrationStatus s) {
    switch (s) {
      case WsIntegrationStatus.connected:
        return GitProviderConnectionStatus.connected;
      case WsIntegrationStatus.disconnected:
        return GitProviderConnectionStatus.disconnected;
      case WsIntegrationStatus.error:
        return GitProviderConnectionStatus.error;
      case WsIntegrationStatus.pending:
        return GitProviderConnectionStatus.pending;
    }
  }
}

/// Long-lived контроллер экрана Git Integrations.
final gitIntegrationsControllerProvider =
    NotifierProvider<GitIntegrationsController, GitIntegrationsState>(
      GitIntegrationsController.new,
    );
