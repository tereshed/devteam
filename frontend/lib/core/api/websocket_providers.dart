import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

part 'websocket_providers.g.dart';

/// Таймауты и лимиты WebSocket-клиента; переопределите в `main`/override для staging.
@Riverpod(keepAlive: true)
WsConfig wsConfig(Ref ref) {
  return const WsConfig();
}

/// WebSocket-сервис на время жизни scope-провайдера; сокет закрывается при dispose.
@Riverpod(keepAlive: false)
WebSocketService webSocketService(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  final baseUrl = dio.options.baseUrl;
  final config = ref.watch(wsConfigProvider);

  final service = WebSocketService(
    baseUrl: baseUrl,
    config: config,
    channelFactory: (uri, {protocols}) =>
        WebSocketChannel.connect(uri, protocols: protocols),
    authProvider: () async {
      final token = ref.read(accessTokenProvider);
      if (token != null && token.isNotEmpty) {
        return WsAuth.bearer(token);
      }
      return const WsAuth.none();
    },
  );
  ref.onDispose(service.dispose);
  return service;
}
