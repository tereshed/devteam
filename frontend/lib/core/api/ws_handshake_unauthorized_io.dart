import 'dart:io' as io;

import 'package:web_socket_channel/web_socket_channel.dart';

/// Только явный отказ upgrade с HTTP 401 (сообщение dart:io), без substring по «401» в hostname.
final RegExp _wsUpgradeHttp401 = RegExp(
  r'HTTP status code:\s*401\b',
  caseSensitive: false,
);

bool wsHandshakeIndicatesHttpUnauthorized(Object error) {
  final objs = <Object?>[error];
  if (error is WebSocketChannelException) {
    objs.add(error.inner);
    if (error.message != null && error.message!.isNotEmpty) {
      objs.add(error.message);
    }
  }
  for (final o in objs) {
    if (o is io.WebSocketException) {
      if (_wsUpgradeHttp401.hasMatch(o.message)) {
        return true;
      }
    }
    if (o is String && _wsUpgradeHttp401.hasMatch(o)) {
      return true;
    }
  }
  return false;
}
