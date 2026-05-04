@TestOn('browser')

library;

import 'dart:async';
import 'dart:math';
import 'dart:typed_data';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:web_socket/web_socket.dart';
import 'package:web_socket_channel/adapter_web_socket_channel.dart';

const _pid = '550e8400-e29b-41d4-a716-446655440000';

/// Минимальный [WebSocket] для [AdapterWebSocketChannel] (дублирует VM-тест).
class _FakeWebSocket implements WebSocket {
  _FakeWebSocket({required this.protocol});
  @override
  final String protocol;

  final StreamController<WebSocketEvent> _ctrl =
      StreamController<WebSocketEvent>(sync: true);

  @override
  Stream<WebSocketEvent> get events => _ctrl.stream;

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
  group('WebSocketService (web)', () {
    test(
      'ch.ready: ошибка без типизированного 401 → transient, не authExpired',
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
                  Future<WebSocket>.error(Exception('handshake failed')),
                );
              }
              fake = _FakeWebSocket(protocol: '');
              return AdapterWebSocketChannel(Future.value(fake));
            },
            authProvider: () async => const WsAuth.bearer('stale-jwt'),
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
      },
    );
  });
}
