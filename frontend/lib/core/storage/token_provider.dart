import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:frontend/core/storage/token_storage.dart';

part 'token_provider.g.dart';

/// Provider для access token
///
/// Управляет состоянием токена авторизации.
/// Используется для синхронного доступа к токену в DioClient.
@Riverpod(keepAlive: true)
class AccessToken extends _$AccessToken {
  @override
  String? build() {
    return null;
  }

  /// Инициализировать токен из storage
  Future<void> init() async {
    final token = await TokenStorage.getAccessToken();
    state = token;
  }

  /// Установить токен
  Future<void> setToken(String token) async {
    await TokenStorage.saveAccessToken(token);
    state = token;
  }

  /// Очистить токен
  Future<void> clear() async {
    await TokenStorage.clearTokens();
    state = null;
  }
}
