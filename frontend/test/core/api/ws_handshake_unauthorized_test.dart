@TestOn('vm')

import 'dart:io' as io;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/ws_handshake_unauthorized.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

void main() {
  test('альтернативная формулировка Dart: «was not upgraded…: HTTP status code: 401»',
      () {
    final e = io.WebSocketException(
      'Connection to https://example.com was not upgraded to websocket: '
      'HTTP status code: 401',
    );
    expect(wsHandshakeIndicatesHttpUnauthorized(e), isTrue);
  });

  test('WebSocketException без статуса → false', () {
    expect(
      wsHandshakeIndicatesHttpUnauthorized(
        io.WebSocketException('Connection failed'),
      ),
      isFalse,
    );
  });

  test('Exception с «401» в тексте без шаблона HTTP status → false', () {
    expect(
      wsHandshakeIndicatesHttpUnauthorized(
        Exception('something with 401 in name'),
      ),
      isFalse,
    );
  });

  test('WebSocketChannelException: только message с 401 Unauthorized', () {
    final w = WebSocketChannelException(
      'HTTP status code: 401 Unauthorized',
    );
    expect(wsHandshakeIndicatesHttpUnauthorized(w), isTrue);
  });

  test('явный HTTP 401 в WebSocketException', () {
    final e = io.WebSocketException(
      'Connection was not upgraded to websocket, HTTP status code: 401',
    );
    expect(wsHandshakeIndicatesHttpUnauthorized(e), isTrue);
  });

  test('401 в тексте Unauthorized', () {
    final e = io.WebSocketException(
      'Connection was not upgraded to websocket, HTTP status code: 401 Unauthorized',
    );
    expect(wsHandshakeIndicatesHttpUnauthorized(e), isTrue);
  });

  test('hostname auth-401-staging без HTTP status 401 → false', () {
    expect(
      wsHandshakeIndicatesHttpUnauthorized(
        Exception('Connection failed: server "auth-401-staging" timed out'),
      ),
      isFalse,
    );
  });

  test('DNS / generic socket → false', () {
    expect(
      wsHandshakeIndicatesHttpUnauthorized(
        const io.SocketException('Failed host lookup: example.invalid'),
      ),
      isFalse,
    );
  });

  test('WebSocketChannelException.from(inner) с 401', () {
    final inner = io.WebSocketException(
      'Connection was not upgraded to websocket, HTTP status code: 401',
    );
    final w = WebSocketChannelException.from(inner);
    expect(wsHandshakeIndicatesHttpUnauthorized(w), isTrue);
  });
}
