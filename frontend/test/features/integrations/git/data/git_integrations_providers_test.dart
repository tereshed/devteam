// @Tags(['unit'])

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_repository.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';

class _FakeRepo implements GitIntegrationsRepository {
  _FakeRepo({
    Map<GitIntegrationProvider, GitProviderConnection>? snapshot,
    this.statusGate,
    this.initResult,
    this.initThrows,
    this.initGate,
    this.revokeReturn = false,
    this.revokeThrows,
  }) : _snapshot =
           snapshot ?? <GitIntegrationProvider, GitProviderConnection>{};

  Map<GitIntegrationProvider, GitProviderConnection> _snapshot;
  int statusCallCount = 0;
  int initCallCount = 0;
  int revokeCallCount = 0;
  Completer<void>? statusGate;
  Completer<void>? initGate;

  GitOAuthInitResult? initResult;
  Object? initThrows;
  bool revokeReturn;
  Object? revokeThrows;

  void setSnapshot(Map<GitIntegrationProvider, GitProviderConnection> next) {
    _snapshot = next;
  }

  @override
  Future<GitProviderConnection> fetchStatus(
    GitIntegrationProvider provider, {
    cancelToken,
  }) async {
    statusCallCount++;
    if (statusGate != null) {
      await statusGate!.future;
    }
    return _snapshot[provider] ??
        GitProviderConnection(
          provider: provider,
          status: GitProviderConnectionStatus.disconnected,
        );
  }

  @override
  Future<GitOAuthInitResult> init(
    GitIntegrationProvider provider, {
    required String redirectUri,
    String? host,
    String? byoClientId,
    String? byoClientSecret,
    cancelToken,
  }) async {
    initCallCount++;
    if (initGate != null) {
      await initGate!.future;
    }
    if (initThrows != null) {
      throw initThrows!;
    }
    return initResult ??
        const GitOAuthInitResult(
          authorizeUrl: 'https://example.com/oauth',
          state: 'st',
        );
  }

  @override
  Future<bool> revoke(GitIntegrationProvider provider, {cancelToken}) async {
    revokeCallCount++;
    if (revokeThrows != null) {
      throw revokeThrows!;
    }
    return revokeReturn;
  }

  @override
  dynamic noSuchMethod(Invocation invocation) {
    throw UnimplementedError(
      'Метод ${invocation.memberName} не реализован в _FakeRepo',
    );
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

  void add(WsClientEvent ev) => _ctrl.add(ev);

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

/// Прокачать microtask-очередь — `build()` контроллера авто-запускает `refresh()`
/// через `scheduleMicrotask`, и нам нужно подождать, чтобы он завершился, прежде
/// чем делать assertion'ы.
Future<void> _pumpMicrotasks([int rounds = 8]) async {
  for (var i = 0; i < rounds; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  group('GitIntegrationsController', () {
    test('refresh() — собирает оба провайдера в Map', () async {
      final repo = _FakeRepo(
        snapshot: {
          GitIntegrationProvider.github: const GitProviderConnection(
            provider: GitIntegrationProvider.github,
            status: GitProviderConnectionStatus.connected,
            accountLogin: 'octocat',
          ),
          GitIntegrationProvider.gitlab: const GitProviderConnection(
            provider: GitIntegrationProvider.gitlab,
            status: GitProviderConnectionStatus.disconnected,
          ),
        },
      );
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);
      container.read(gitIntegrationsControllerProvider.notifier);
      // Авто-refresh из build() — ждём, пока завершится.
      await _pumpMicrotasks();

      final s = container.read(gitIntegrationsControllerProvider);
      expect(s.isLoading, isFalse);
      expect(
        s.connections[GitIntegrationProvider.github]?.status,
        GitProviderConnectionStatus.connected,
      );
      expect(
        s.connections[GitIntegrationProvider.github]?.accountLogin,
        'octocat',
      );
      expect(
        s.connections[GitIntegrationProvider.gitlab]?.status,
        GitProviderConnectionStatus.disconnected,
      );
    });

    test(
      'IntegrationConnectionChanged{provider:github} мутирует state',
      () async {
        final repo = _FakeRepo();
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        // Создаём контроллер.
        container.read(gitIntegrationsControllerProvider.notifier);

        ws.add(
          WsClientEvent.server(
            WsServerEvent.integrationStatus(
              WsIntegrationStatusEvent(
                ts: DateTime.utc(2030, 1, 1),
                v: 1,
                userId: 'user-1',
                provider: 'github',
                status: WsIntegrationStatus.connected,
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);

        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.github]
              ?.status,
          GitProviderConnectionStatus.connected,
        );
      },
    );

    test('чужой провайдер (claude_code_oauth) игнорируется', () async {
      final repo = _FakeRepo();
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);
      container.read(gitIntegrationsControllerProvider.notifier);
      // Дождёмся завершения авто-refresh из build() — после него карта будет
      // содержать оба провайдера с дефолтным `disconnected`.
      await _pumpMicrotasks();
      final beforeConnections =
          Map<GitIntegrationProvider, GitProviderConnection>.from(
            container.read(gitIntegrationsControllerProvider).connections,
          );

      ws.add(
        WsClientEvent.server(
          WsServerEvent.integrationStatus(
            WsIntegrationStatusEvent(
              ts: DateTime.utc(2030, 1, 1),
              v: 1,
              userId: 'user-1',
              provider: 'claude_code_oauth',
              status: WsIntegrationStatus.connected,
            ),
          ),
        ),
      );
      await _pumpMicrotasks();

      expect(
        container.read(gitIntegrationsControllerProvider).connections,
        beforeConnections,
        reason: 'WS-событие чужого провайдера не должно ничего менять в state',
      );
    });

    test(
      'reconnect (transient failure → server event) запускает повторный GET /status',
      () async {
        final repo = _FakeRepo();
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);

        await c.refresh();
        final initialCalls = repo.statusCallCount;

        // Транзитный сбой WS — controller помечает «надо сделать resync».
        ws.add(
          const WsClientEvent.serviceFailure(WsServiceFailure.transient()),
        );
        await Future<void>.delayed(Duration.zero);
        expect(
          repo.statusCallCount,
          initialCalls,
          reason: 'на сбое НЕ должен сразу пере-фетчить',
        );

        // Любой server-event = «сокет снова жив» → re-sync.
        ws.add(
          WsClientEvent.server(
            WsServerEvent.integrationStatus(
              WsIntegrationStatusEvent(
                ts: DateTime.utc(2030, 1, 1),
                v: 1,
                userId: 'user-1',
                provider: 'github',
                status: WsIntegrationStatus.pending,
              ),
            ),
          ),
        );
        await Future<void>.delayed(const Duration(milliseconds: 5));

        expect(
          repo.statusCallCount,
          greaterThan(initialCalls),
          reason: 'после reconnect должен быть второй GET /status',
        );
      },
    );

    test('pending → connected: после обрыва backend прислал connected, '
        'resync переводит state из pending в connected', () async {
      final repo = _FakeRepo();
      // Изначально оба disconnected (пустой snapshot).
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);
      final c = container.read(gitIntegrationsControllerProvider.notifier);

      await c.refresh();
      // Ставим локально pending (как делает UI после init).
      c.applyLocal(
        const GitProviderConnection(
          provider: GitIntegrationProvider.github,
          status: GitProviderConnectionStatus.pending,
        ),
      );
      expect(
        container
            .read(gitIntegrationsControllerProvider)
            .connections[GitIntegrationProvider.github]
            ?.status,
        GitProviderConnectionStatus.pending,
      );

      // WS обрывается во время OAuth.
      ws.add(const WsClientEvent.serviceFailure(WsServiceFailure.transient()));
      await Future<void>.delayed(Duration.zero);

      // Пока был обрыв — backend завершил OAuth, в БД connected.
      repo.setSnapshot({
        GitIntegrationProvider.github: const GitProviderConnection(
          provider: GitIntegrationProvider.github,
          status: GitProviderConnectionStatus.connected,
          accountLogin: 'octocat',
        ),
      });

      // Реконнект: приходит любой server-event → trigger resync.
      // Используем «чужой» провайдер, чтобы убедиться, что ресинк не зависит
      // от содержимого события.
      ws.add(
        WsClientEvent.server(
          WsServerEvent.integrationStatus(
            WsIntegrationStatusEvent(
              ts: DateTime.utc(2030, 1, 1),
              v: 1,
              userId: 'user-1',
              provider: 'claude_code_oauth',
              status: WsIntegrationStatus.connected,
            ),
          ),
        ),
      );
      await Future<void>.delayed(const Duration(milliseconds: 5));

      expect(
        container
            .read(gitIntegrationsControllerProvider)
            .connections[GitIntegrationProvider.github]
            ?.status,
        GitProviderConnectionStatus.connected,
        reason: 'resync должен подтянуть актуальный connected из БД',
      );
      expect(
        container
            .read(gitIntegrationsControllerProvider)
            .connections[GitIntegrationProvider.github]
            ?.accountLogin,
        'octocat',
      );
    });

    test(
      'refresh() в полёте не затирает WS-обновление (version-guard)',
      () async {
        final gate = Completer<void>();
        final repo = _FakeRepo(
          snapshot: {
            GitIntegrationProvider.github: const GitProviderConnection(
              provider: GitIntegrationProvider.github,
              status: GitProviderConnectionStatus.disconnected,
            ),
            GitIntegrationProvider.gitlab: const GitProviderConnection(
              provider: GitIntegrationProvider.gitlab,
              status: GitProviderConnectionStatus.disconnected,
            ),
          },
          statusGate: gate,
        );
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);

        final c = container.read(gitIntegrationsControllerProvider.notifier);

        // Стартуем refresh — он висит на gate.
        final refreshFuture = c.refresh();
        await Future<void>.delayed(Duration.zero);

        // Пока REST висит — WS-событие "connected" для github.
        ws.add(
          WsClientEvent.server(
            WsServerEvent.integrationStatus(
              WsIntegrationStatusEvent(
                ts: DateTime.utc(2030, 1, 1),
                v: 1,
                userId: 'user-1',
                provider: 'github',
                status: WsIntegrationStatus.connected,
              ),
            ),
          ),
        );
        await Future<void>.delayed(Duration.zero);
        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.github]
              ?.status,
          GitProviderConnectionStatus.connected,
        );

        // Размораживаем REST — он вернёт устаревший "disconnected".
        gate.complete();
        await refreshFuture;

        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.github]
              ?.status,
          GitProviderConnectionStatus.connected,
          reason: 'устаревший REST-снимок не должен перезатереть свежий WS',
        );
      },
    );

    test('parse/auth-event без server-event resync не вызывает', () async {
      final repo = _FakeRepo();
      final ws = _FakeWebSocketService();
      final container = _container(repo: repo, ws: ws);
      addTearDown(container.dispose);
      addTearDown(ws.close);
      final c = container.read(gitIntegrationsControllerProvider.notifier);

      await c.refresh();
      final before = repo.statusCallCount;
      ws.add(const WsClientEvent.parseError(WsParseError(message: 'x')));
      await Future<void>.delayed(Duration.zero);
      expect(repo.statusCallCount, before);
    });

    group('initConnection / disconnect / rollbackToDisconnected', () {
      test(
        'initConnection — успех: дёргает repo.init, ставит pending, возвращает URL',
        () async {
          final repo = _FakeRepo(
            initResult: const GitOAuthInitResult(
              authorizeUrl: 'https://gh/authorize?x=1',
              state: 'st',
            ),
          );
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);

          final url = await c.initConnection(
            GitIntegrationProvider.github,
            redirectUri: 'https://example.app/cb',
          );

          expect(url, 'https://gh/authorize?x=1');
          expect(repo.initCallCount, 1);
          expect(
            container
                .read(gitIntegrationsControllerProvider)
                .connections[GitIntegrationProvider.github]
                ?.status,
            GitProviderConnectionStatus.pending,
          );
        },
      );

      test(
        'initConnection — exception + updateStateOnError=true: state error+rethrow',
        () async {
          final repo = _FakeRepo(
            initThrows: const GitIntegrationsException(
              message: 'invalid host',
              errorCode: 'invalid_host',
              statusCode: 400,
            ),
          );
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);

          await expectLater(
            c.initConnection(
              GitIntegrationProvider.gitlab,
              redirectUri: 'https://example.app/cb',
            ),
            throwsA(isA<GitIntegrationsException>()),
          );
          final conn = container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.gitlab];
          expect(conn?.status, GitProviderConnectionStatus.error);
          expect(conn?.reason, 'invalid_host');
        },
      );

      test(
        'initConnection — exception + updateStateOnError=false: state не трогаем',
        () async {
          final repo = _FakeRepo(
            initThrows: const GitIntegrationsException(
              message: 'invalid host',
              errorCode: 'invalid_host',
              statusCode: 400,
            ),
          );
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);
          await c.refresh(); // snapshot пустой → оба disconnected

          await expectLater(
            c.initConnection(
              GitIntegrationProvider.gitlab,
              redirectUri: 'https://example.app/cb',
              updateStateOnError: false,
            ),
            throwsA(isA<GitIntegrationsException>()),
          );
          expect(
            container
                .read(gitIntegrationsControllerProvider)
                .connections[GitIntegrationProvider.gitlab]
                ?.status,
            GitProviderConnectionStatus.disconnected,
            reason: 'BYO-диалог сам отрисует ошибку — карточку не трогаем',
          );
        },
      );

      test('disconnect — успех (remote_revoke_failed=false)', () async {
        final repo = _FakeRepo(revokeReturn: false);
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);

        await c.disconnect(GitIntegrationProvider.github);

        expect(repo.revokeCallCount, 1);
        final conn = container
            .read(gitIntegrationsControllerProvider)
            .connections[GitIntegrationProvider.github];
        expect(conn?.status, GitProviderConnectionStatus.disconnected);
        expect(conn?.remoteRevokeFailed, isFalse);
      });

      test(
        'disconnect — remote_revoke_failed=true пробрасывается в state',
        () async {
          final repo = _FakeRepo(revokeReturn: true);
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);

          await c.disconnect(GitIntegrationProvider.github);

          expect(
            container
                .read(gitIntegrationsControllerProvider)
                .connections[GitIntegrationProvider.github]
                ?.remoteRevokeFailed,
            isTrue,
          );
        },
      );

      test('disconnect — exception → background refresh()', () async {
        final repo = _FakeRepo(
          revokeThrows: const GitIntegrationsException(message: 'boom'),
        );
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);
        await c.refresh();
        final statusCallsBefore = repo.statusCallCount;

        await c.disconnect(GitIntegrationProvider.github);
        // refresh() запущен через unawaited — дождёмся микротасок.
        await Future<void>.delayed(const Duration(milliseconds: 5));

        expect(
          repo.statusCallCount,
          greaterThan(statusCallsBefore),
          reason: 'после revoke-ошибки должен пройти refresh()',
        );
      });

      test(
        'rollbackToDisconnected — снимает pending обратно в disconnected',
        () async {
          final repo = _FakeRepo();
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);

          c.applyLocal(
            const GitProviderConnection(
              provider: GitIntegrationProvider.github,
              status: GitProviderConnectionStatus.pending,
            ),
          );
          c.rollbackToDisconnected(GitIntegrationProvider.github);

          expect(
            container
                .read(gitIntegrationsControllerProvider)
                .connections[GitIntegrationProvider.github]
                ?.status,
            GitProviderConnectionStatus.disconnected,
          );
        },
      );

      test(
        'initConnection — сразу выставляет pending (до сетевого запроса)',
        () async {
          final initGate = Completer<void>();
          final repo = _FakeRepo(initGate: initGate);
          final ws = _FakeWebSocketService();
          final container = _container(repo: repo, ws: ws);
          addTearDown(container.dispose);
          addTearDown(ws.close);
          final c = container.read(gitIntegrationsControllerProvider.notifier);
          // Дождёмся авто-refresh из build(), иначе он гонится с pending.
          await _pumpMicrotasks();

          // Стартуем init — он висит на gate.
          final initFuture = c.initConnection(
            GitIntegrationProvider.github,
            redirectUri: 'https://example.app/cb',
          );
          await Future<void>.delayed(Duration.zero);

          // Кнопка уже должна быть заблокирована, пока летит запрос.
          expect(
            container
                .read(gitIntegrationsControllerProvider)
                .connections[GitIntegrationProvider.github]
                ?.status,
            GitProviderConnectionStatus.pending,
            reason:
                'pending выставляется ДО await — иначе кнопку можно накликать',
          );

          initGate.complete();
          await initFuture;
        },
      );

      test('initConnection — race: откатили pending до возврата init → '
          'cancelled_locally вместо вечного pending', () async {
        final initGate = Completer<void>();
        final repo = _FakeRepo(initGate: initGate);
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);
        // Дождёмся авто-refresh из build(), иначе он гонится с pending.
        await _pumpMicrotasks();

        // 1. Стартуем init — он висит, pending уже выставлен.
        final initFuture = c.initConnection(
          GitIntegrationProvider.gitlab,
          redirectUri: 'https://example.app/cb',
          host: 'https://gl.acme.corp',
          byoClientId: 'cid',
          byoClientSecret: 'sec',
          updateStateOnError: false,
        );
        await Future<void>.delayed(Duration.zero);
        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.gitlab]
              ?.status,
          GitProviderConnectionStatus.pending,
        );

        // 2. Юзер закрыл BYO-диалог → dispose позвал rollback.
        c.rollbackToDisconnected(GitIntegrationProvider.gitlab);

        // 3. Размораживаем init — он успешно вернул URL, но контроллер
        //    должен заметить race и бросить cancelled_locally.
        initGate.complete();
        await expectLater(
          initFuture,
          throwsA(
            isA<GitIntegrationsException>().having(
              (e) => e.message,
              'message',
              'cancelled_locally',
            ),
          ),
        );

        // Стейт остался disconnected — не зависли в pending.
        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.gitlab]
              ?.status,
          GitProviderConnectionStatus.disconnected,
          reason: 'race-detection должен сохранить откаченный stat',
        );
      });

      test('initConnection — generic exception (не GitIntegrationsException) → '
          'state error(internal_error) + rethrow', () async {
        final repo = _FakeRepo(initThrows: StateError('boom'));
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);

        await expectLater(
          c.initConnection(
            GitIntegrationProvider.github,
            redirectUri: 'https://example.app/cb',
          ),
          throwsA(isA<StateError>()),
        );
        final conn = container
            .read(gitIntegrationsControllerProvider)
            .connections[GitIntegrationProvider.github];
        expect(conn?.status, GitProviderConnectionStatus.error);
        expect(conn?.reason, 'internal_error');
      });

      test('initConnection — generic exception с updateStateOnError=false → '
          'rollback в disconnected, не висит в pending', () async {
        final repo = _FakeRepo(initThrows: StateError('boom'));
        final ws = _FakeWebSocketService();
        final container = _container(repo: repo, ws: ws);
        addTearDown(container.dispose);
        addTearDown(ws.close);
        final c = container.read(gitIntegrationsControllerProvider.notifier);

        await expectLater(
          c.initConnection(
            GitIntegrationProvider.gitlab,
            redirectUri: 'https://example.app/cb',
            host: 'https://gl.acme.corp',
            updateStateOnError: false,
          ),
          throwsA(isA<StateError>()),
        );
        expect(
          container
              .read(gitIntegrationsControllerProvider)
              .connections[GitIntegrationProvider.gitlab]
              ?.status,
          GitProviderConnectionStatus.disconnected,
        );
      });
    });
  });
}
