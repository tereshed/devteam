import 'dart:async';
import 'dart:math';
import 'dart:typed_data';

import 'package:clock/clock.dart';
import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:web_socket/web_socket.dart';
import 'package:web_socket_channel/adapter_web_socket_channel.dart';

const _pid = '550e8400-e29b-41d4-a716-446655440000';
const _pidB = '660e8400-e29b-41d4-a716-446655440001';

String _errorEnvelope(String code) =>
    '{"type":"error","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"$_pid","data":{"code":"$code","message":"m"}}';

String _taskStatus(String pid) =>
    '{"type":"task_status","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"$pid","data":{"task_id":"770e8400-e29b-41d4-a716-446655440002","previous_status":"a","status":"b"}}';

/// Минимальная реализация [WebSocket] для [AdapterWebSocketChannel] без TCP.
class _FakeWebSocket implements WebSocket {
  _FakeWebSocket({required this.protocol});
  @override
  final String protocol;

  final StreamController<WebSocketEvent> _ctrl =
      StreamController<WebSocketEvent>(sync: true);

  @override
  Stream<WebSocketEvent> get events => _ctrl.stream;

  void pushText(String s) {
    if (!_ctrl.isClosed) {
      _ctrl.add(TextDataReceived(s));
    }
  }

  void endPeer([int? code, String reason = '']) {
    if (!_ctrl.isClosed) {
      _ctrl.add(CloseReceived(code, reason));
      unawaited(_ctrl.close());
    }
  }

  @override
  void sendText(String s) {}

  @override
  void sendBytes(Uint8List b) {}

  @override
  Future<void> close([int? code, String? reason]) async {
    if (!_ctrl.isClosed) {
      await _ctrl.close();
    }
  }
}

void main() {
  group('WebSocketService', () {
    test('dispose затем connect → StateError', () {
      late _FakeWebSocket fake;
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: protocols?.first ?? '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.dispose();
      expect(() => svc.connect(_pid), throwsStateError);
    });

    test('pause → disconnect → connect не бросает (paused сброшен)', () async {
      late _FakeWebSocket fake;
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      svc.pause();
      svc.disconnect();
      expect(() => svc.connect(_pid), returnsNormally);
      await Future<void>.delayed(Duration.zero);
      svc.dispose();
    });

    test('смена projectId в open → transient switching project', () async {
      var factories = 0;
      late _FakeWebSocket fake;
      final buf = <WsClientEvent>[];
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          factories++;
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.events.listen(buf.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      svc.connect(_pidB);
      await Future<void>.delayed(Duration.zero);
      expect(factories, 2);
      expect(
        buf.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              transient: (x) => '$x' == 'switching project',
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      svc.dispose();
    });

    test('невалидный projectId → ArgumentError', () {
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          final f = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(f));
        },
        authProvider: () async => const WsAuth.none(),
      );
      expect(() => svc.connect('not-uuid'), throwsArgumentError);
      svc.dispose();
    });

    test('connectTimeout: >10s виртуального времени → transient', () {
      FakeAsync().run((async) {
        final c = Completer<WebSocket>();
        try {
          final buf = <WsClientEvent>[];
          final svc = WebSocketService(
            baseUrl: 'http://localhost:8080/api/v1',
            channelFactory: (uri, {protocols}) =>
                AdapterWebSocketChannel(c.future),
            authProvider: () async => const WsAuth.none(),
          );
          svc.events.listen(buf.add);
          svc.connect(_pid);
          async.elapse(const Duration(seconds: 11));
          async.flushMicrotasks();
          final hit = buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                transient: (x) => '$x'.contains('connectTimeout'),
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          );
          expect(hit, isTrue);
          svc.dispose();
        } finally {
          if (!c.isCompleted) {
            c.completeError(Exception('cancelled'));
          }
        }
      });
    });

    test('backoffUpperExclusiveMsForTesting', () {
      const c = WsConfig();
      expect(backoffUpperExclusiveMsForTesting(c, 0), 500);
      expect(backoffUpperExclusiveMsForTesting(c, 5), 16000);
      expect(backoffUpperExclusiveMsForTesting(c, 24), 30000);
      expect(backoffUpperExclusiveMsForTesting(c, 100), 30000);
    });

    test('circuit breaker: N parse errors → protocolBroken', () async {
      var tick = 0;
      final clock = Clock(() {
        tick++;
        return DateTime.utc(2020).add(Duration(milliseconds: 100 * tick));
      });
      late _FakeWebSocket fake;
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        clock: clock,
        config: const WsConfig(
          circuitParseErrors: 5,
          circuitWindow: Duration(seconds: 10),
        ),
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      final seen = <WsClientEvent>[];
      final sub = svc.events.listen(seen.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      for (var i = 0; i < 5; i++) {
        fake.pushText('not-json-$i');
        await Future<void>.delayed(Duration.zero);
      }
      await Future<void>.delayed(const Duration(milliseconds: 50));
      expect(
        seen.where(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              protocolBroken: () => true,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isNotEmpty,
      );
      await sub.cancel();
      svc.dispose();
    });

    test('circuit breaker: после protocolBroken нет авто-reconnect', () {
      FakeAsync().run((async) {
        var tick = 0;
        final clock = Clock(() {
          tick++;
          return DateTime.utc(2020).add(Duration(milliseconds: 100 * tick));
        });
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          clock: clock,
          config: const WsConfig(
            circuitParseErrors: 5,
            circuitWindow: Duration(seconds: 10),
          ),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen((_) {});
        svc.connect(_pid);
        async.flushMicrotasks();
        for (var i = 0; i < 5; i++) {
          fake.pushText('not-json-$i');
        }
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(seconds: 60));
        async.flushMicrotasks();
        expect(factories, 1);
        svc.dispose();
      });
    });

    test('idleTimeout: >65s без кадров → reconnect path (transient)', () {
      FakeAsync().run((async) {
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        final buf = <WsClientEvent>[];
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        fake.pushText(
          '{"type":"task_status","v":1,"ts":"2026-01-01T00:00:00.000Z","project_id":"$_pid","data":{"task_id":"660e8400-e29b-41d4-a716-446655440001","previous_status":"a","status":"b"}}',
        );
        async.flushMicrotasks();
        async.elapse(const Duration(seconds: 66));
        async.flushMicrotasks();
        final idleHit = buf.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              transient: (x) => '$x'.contains('idleTimeout'),
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        );
        expect(idleHit, isTrue);
        svc.dispose();
      });
    });

    test('disconnect отменяет backoff (FakeAsync)', () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          random: Random(0),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.connect(_pid);
        async.flushMicrotasks();
        expect(factories, 1);
        scheduleMicrotask(() => fake.endPeer(1006));
        async.flushMicrotasks();
        svc.disconnect();
        async.elapse(const Duration(seconds: 120));
        async.flushMicrotasks();
        expect(factories, 1);
        svc.dispose();
      });
    });

    test('dispose затем disconnect → StateError', () {
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          final f = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(f));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.dispose();
      expect(() => svc.disconnect(), throwsStateError);
    });

    test('connect идемпотентен при connecting (тот же projectId)', () async {
      var factories = 0;
      final c = Completer<WebSocket>();
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          factories++;
          return AdapterWebSocketChannel(c.future);
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      expect(factories, 1);
      c.complete(_FakeWebSocket(protocol: ''));
      await Future<void>.delayed(Duration.zero);
      svc.dispose();
    });

    test('смена projectId: завершение старого сокета не ломает активную сессию',
        () async {
      var factories = 0;
      Completer<WebSocket>? first;
      late _FakeWebSocket second;
      final buf = <WsClientEvent>[];
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          factories++;
          if (factories == 1) {
            first = Completer<WebSocket>();
            return AdapterWebSocketChannel(first!.future);
          }
          second = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(second));
        },
        authProvider: () async => const WsAuth.none(),
      );
      final sub = svc.events.listen(buf.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      svc.connect(_pidB);
      await Future<void>.delayed(Duration.zero);
      expect(factories, 2);
      first!.complete(_FakeWebSocket(protocol: ''));
      await Future<void>.delayed(Duration.zero);
      second.pushText(_taskStatus(_pidB));
      await Future<void>.delayed(Duration.zero);
      expect(
        buf.any(
          (e) => e.maybeWhen(
            server: (_) => true,
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      await sub.cancel();
      svc.dispose();
    });

    test('subprotocol mismatch: в событии нет секрета, есть bearer.<jwt>', () async {
      late _FakeWebSocket fake;
      final seen = <WsClientEvent>[];
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: 'bearer.wrong');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.bearer('ultra-secret-jwt'),
      );
      final sub = svc.events.listen(seen.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      WsSubprotocolMismatch? info;
      for (final e in seen) {
        e.maybeWhen(
          subprotocolMismatch: (m) => info = m,
          orElse: () {},
        );
      }
      expect(info, isNotNull);
      final m = info!;
      expect(m.expected, 'bearer.<jwt>');
      expect(m.received, 'bearer.***');
      expect(
        '${m.expected}${m.received}',
        isNot(contains('ultra-secret')),
      );
      expect(
        seen.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              policySubprotocolMismatch: () => true,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      await sub.cancel();
      svc.dispose();
    });

    test('ch.ready: не-401 → transient, без authExpired; затем reconnect',
        () {
      FakeAsync().run((async) {
        var wave = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          random: Random(0),
          channelFactory: (uri, {protocols}) {
            wave++;
            if (wave == 1) {
              return AdapterWebSocketChannel(
                Future<WebSocket>.error(Exception('connection refused')),
              );
            }
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                transient: (_) => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isTrue,
        );
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                authExpired: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        async.elapse(const Duration(milliseconds: 455));
        async.flushMicrotasks();
        expect(wave, greaterThanOrEqualTo(2));
        svc.dispose();
      });
    });

    test('close 4401 → authExpired + authFailure', () async {
      late _FakeWebSocket fake;
      final buf = <WsClientEvent>[];
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.events.listen(buf.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      fake.endPeer(4401, 'token expired');
      await Future<void>.delayed(Duration.zero);
      expect(
        buf.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              authExpired: () => true,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      expect(
        buf.any(
          (e) => e.maybeWhen(
            authFailure: (a) =>
                a.closeCode == 4401 && a.closeReason == 'token expired',
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      svc.dispose();
    });

    test('close 1008 → policyCloseCode (терминал)', () async {
      late _FakeWebSocket fake;
      final buf = <WsClientEvent>[];
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.events.listen(buf.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      fake.endPeer(1008);
      await Future<void>.delayed(Duration.zero);
      expect(
        buf.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              policyCloseCode: (c) => c == 1008,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      svc.dispose();
    });

    test(
        '4429 после успешного open: снова tooManyConnections, не мгновенный терминал',
        () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        async.elapse(const Duration(seconds: 61));
        async.flushMicrotasks();
        expect(factories, 2);
        fake.pushText(_taskStatus(_pid));
        async.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        expect(
          buf.where(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnections: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          hasLength(2),
        );
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnectionsTerminal: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        svc.dispose();
      });
    });

    test(
        'pause после 4429 → resume → connect сбрасывает счётчик; второй 4429 не terminal',
        () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        expect(
          buf.where(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnections: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          hasLength(1),
        );
        svc.pause();
        async.flushMicrotasks();
        unawaited(svc.resume());
        async.flushMicrotasks();
        expect(factories, 2);
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        expect(
          buf.where(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnections: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          hasLength(2),
        );
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnectionsTerminal: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        svc.dispose();
      });
    });

    test('close 4429 → tooManyConnections, retry 60s, второй 4429 без envelope → terminal',
        () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        expect(factories, 1);
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        expect(
          buf.where(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnections: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          hasLength(1),
        );
        async.elapse(const Duration(seconds: 59));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(seconds: 2));
        async.flushMicrotasks();
        expect(factories, 2);
        scheduleMicrotask(() => fake.endPeer(4429));
        async.flushMicrotasks();
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnectionsTerminal: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isTrue,
        );
        async.elapse(const Duration(seconds: 120));
        async.flushMicrotasks();
        expect(factories, 2);
        svc.dispose();
      });
    });

    test(
        '4429: после ≥30s на сокете без envelope второй 4429 не терминал '
        '(сброс счётчика по stable open)',
        () {
      FakeAsync().run((fa) {
        var factories = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          clock: Clock(() => DateTime.utc(2020, 1, 1).add(fa.elapsed)),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        fa.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(4429));
        fa.flushMicrotasks();
        fa.elapse(const Duration(seconds: 61));
        fa.flushMicrotasks();
        expect(factories, 2);
        fa.elapse(const Duration(seconds: 31));
        fa.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(4429));
        fa.flushMicrotasks();
        expect(
          buf.where(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnections: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          hasLength(2),
        );
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnectionsTerminal: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        fa.elapse(const Duration(seconds: 61));
        fa.flushMicrotasks();
        expect(factories, 3);
        scheduleMicrotask(() => fake.endPeer(4429));
        fa.flushMicrotasks();
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                tooManyConnectionsTerminal: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isTrue,
        );
        svc.dispose();
      });
    });

    test('первый envelope сбрасывает backoff attempt (Random(0))', () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          random: Random(0),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen((_) {});
        svc.connect(_pid);
        async.flushMicrotasks();
        expect(factories, 1);
        scheduleMicrotask(() => fake.endPeer(1006));
        async.flushMicrotasks();
        async.elapse(const Duration(milliseconds: 454));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(milliseconds: 2));
        async.flushMicrotasks();
        expect(factories, 2);

        fake.pushText(_taskStatus(_pid));
        scheduleMicrotask(() => fake.endPeer(1006));
        async.flushMicrotasks();
        async.elapse(const Duration(milliseconds: 8));
        async.flushMicrotasks();
        expect(factories, 2);
        async.elapse(const Duration(milliseconds: 2));
        async.flushMicrotasks();
        expect(factories, 3);

        svc.dispose();
      });
    });

    test(
        'два 1006 подряд без envelope: больший backoff до третьего канала (Random(0))',
        () {
      // Random(0) в dart:math (SDK на момент теста): после 1-го 1006 attempt=1 →
      // nextInt(1000)==455; после 2-го 1006 attempt=2 → nextInt(2000)==1009.
      // При смене алгоритма RNG в major Dart — перепроверить и обновить комментарий/elapse.
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          random: Random(0),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen((_) {});
        svc.connect(_pid);
        async.flushMicrotasks();
        scheduleMicrotask(() => fake.endPeer(1006));
        async.flushMicrotasks();
        async.elapse(const Duration(milliseconds: 456));
        async.flushMicrotasks();
        expect(factories, 2);

        scheduleMicrotask(() => fake.endPeer(1006));
        async.flushMicrotasks();
        async.elapse(const Duration(milliseconds: 15));
        async.flushMicrotasks();
        expect(factories, 2);
        async.elapse(const Duration(milliseconds: 1000));
        async.flushMicrotasks();
        expect(factories, 3);

        svc.dispose();
      });
    });

    test('error forbidden → policyForbidden, без авто-reconnect', () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final buf = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(buf.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        fake.pushText(_errorEnvelope('forbidden'));
        async.flushMicrotasks();
        expect(
          buf.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                policyForbidden: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isTrue,
        );
        expect(factories, 1);
        async.elapse(const Duration(seconds: 30));
        async.flushMicrotasks();
        expect(factories, 1);
        svc.dispose();
      });
    });

    test('error internal_error → reconnect через internalErrorReconnectDelay',
        () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.connect(_pid);
        async.flushMicrotasks();
        fake.pushText(_errorEnvelope('internal_error'));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(seconds: 4));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(seconds: 2));
        async.flushMicrotasks();
        expect(factories, 2);
        svc.dispose();
      });
    });

    test('error server_shutdown: задержка base+jitter (Random(0) = 11280 ms)', () {
      FakeAsync().run((async) {
        var factories = 0;
        late _FakeWebSocket fake;
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          random: Random(0),
          channelFactory: (uri, {protocols}) {
            factories++;
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.connect(_pid);
        async.flushMicrotasks();
        fake.pushText(_errorEnvelope('server_shutdown'));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(milliseconds: 11279));
        async.flushMicrotasks();
        expect(factories, 1);
        async.elapse(const Duration(milliseconds: 2));
        async.flushMicrotasks();
        expect(factories, 2);
        svc.dispose();
      });
    });

    test('pause/resume открывает новый канал', () async {
      var factories = 0;
      late _FakeWebSocket fake;
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        channelFactory: (uri, {protocols}) {
          factories++;
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      expect(factories, 1);
      svc.pause();
      await svc.resume();
      await Future<void>.delayed(Duration.zero);
      expect(factories, 2);
      svc.dispose();
    });

    test('circuit: 49 ошибок парсинга при фиксированных часах — без protocolBroken',
        () async {
      final clock = Clock.fixed(DateTime.utc(2020));
      late _FakeWebSocket fake;
      final svc = WebSocketService(
        baseUrl: 'http://localhost:8080/api/v1',
        clock: clock,
        config: const WsConfig(
          circuitParseErrors: 50,
          circuitWindow: Duration(seconds: 10),
        ),
        channelFactory: (uri, {protocols}) {
          fake = _FakeWebSocket(protocol: '');
          return AdapterWebSocketChannel(Future.value(fake));
        },
        authProvider: () async => const WsAuth.none(),
      );
      final seen = <WsClientEvent>[];
      final sub = svc.events.listen(seen.add);
      svc.connect(_pid);
      await Future<void>.delayed(Duration.zero);
      for (var i = 0; i < 49; i++) {
        fake.pushText('not-json');
        await Future<void>.delayed(Duration.zero);
      }
      expect(
        seen.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              protocolBroken: () => true,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isFalse,
      );
      fake.pushText('not-json');
      await Future<void>.delayed(Duration.zero);
      expect(
        seen.any(
          (e) => e.maybeWhen(
            serviceFailure: (f) => f.maybeWhen(
              protocolBroken: () => true,
              orElse: () => false,
            ),
            orElse: () => false,
          ),
        ),
        isTrue,
      );
      await sub.cancel();
      svc.dispose();
    });

    test('circuit: скользящее окно — после 11s счётчик очищается', () {
      FakeAsync().run((async) {
        final clock = Clock(() => DateTime.utc(2020).add(async.elapsed));
        late _FakeWebSocket fake;
        final seen = <WsClientEvent>[];
        final svc = WebSocketService(
          baseUrl: 'http://localhost:8080/api/v1',
          clock: clock,
          config: const WsConfig(
            circuitParseErrors: 5,
            circuitWindow: Duration(seconds: 10),
          ),
          channelFactory: (uri, {protocols}) {
            fake = _FakeWebSocket(protocol: '');
            return AdapterWebSocketChannel(Future.value(fake));
          },
          authProvider: () async => const WsAuth.none(),
        );
        svc.events.listen(seen.add);
        svc.connect(_pid);
        async.flushMicrotasks();
        for (var i = 0; i < 4; i++) {
          fake.pushText('bad');
          async.elapse(const Duration(milliseconds: 100));
          async.flushMicrotasks();
        }
        expect(
          seen.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                protocolBroken: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        async.elapse(const Duration(seconds: 11));
        async.flushMicrotasks();
        for (var i = 0; i < 4; i++) {
          fake.pushText('bad');
          async.elapse(const Duration(milliseconds: 50));
          async.flushMicrotasks();
        }
        expect(
          seen.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                protocolBroken: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isFalse,
        );
        fake.pushText('bad');
        async.flushMicrotasks();
        expect(
          seen.any(
            (e) => e.maybeWhen(
              serviceFailure: (f) => f.maybeWhen(
                protocolBroken: () => true,
                orElse: () => false,
              ),
              orElse: () => false,
            ),
          ),
          isTrue,
        );
        svc.dispose();
      });
    });
  });
}
