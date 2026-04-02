import 'package:frontend/core/storage/token_provider.dart';
import 'package:frontend/core/storage/token_storage.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/auth/domain/user_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'auth_controller.g.dart';

/// AuthController управляет состоянием авторизации
///
/// Использует AsyncNotifier для управления асинхронным состоянием.
/// Обрабатывает логику входа, регистрации, выхода и получения текущего пользователя.
@riverpod
class AuthController extends _$AuthController {
  @override
  Future<UserModel?> build() async {
    // При инициализации пытаемся получить текущего пользователя
    // если есть сохраненный токен
    final hasTokens = await TokenStorage.hasTokens();
    if (hasTokens) {
      // Инициализируем токен в provider
      await ref.read(accessTokenProvider.notifier).init();

      try {
        final repository = ref.read(authRepositoryProvider);
        final user = await repository.getCurrentUser();
        return user;
      } catch (e) {
        // Если токен невалидный, очищаем storage
        await ref.read(accessTokenProvider.notifier).clear();
        return null;
      }
    }
    return null;
  }

  /// Регистрация нового пользователя
  Future<void> register({
    required String email,
    required String password,
  }) async {
    state = const AsyncValue.loading();

    try {
      final repository = ref.read(authRepositoryProvider);
      final response = await repository.register(
        email: email,
        password: password,
      );

      // Сохраняем токены
      final accessToken = response['access_token'] as String;
      final refreshToken = response['refresh_token'] as String;
      await TokenStorage.saveRefreshToken(refreshToken);

      // Устанавливаем токен в provider
      await ref.read(accessTokenProvider.notifier).setToken(accessToken);

      // Проверяем, что токен установлен
      final tokenCheck = ref.read(accessTokenProvider);
      if (tokenCheck == null || tokenCheck != accessToken) {
        throw Exception('Failed to set access token');
      }

      // Инвалидируем DioClient и AuthRepository чтобы они пересоздались с новым токеном
      ref.invalidate(dioClientProvider);
      ref.invalidate(authRepositoryProvider);

      // Получаем новый repository с обновленным DioClient
      final updatedRepository = ref.read(authRepositoryProvider);

      // Получаем данные пользователя (используем новый DioClient с токеном)
      final user = await updatedRepository.getCurrentUser();
      state = AsyncValue.data(user);
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
      rethrow;
    }
  }

  /// Вход пользователя
  Future<void> login({required String email, required String password}) async {
    state = const AsyncValue.loading();

    try {
      final repository = ref.read(authRepositoryProvider);
      final response = await repository.login(email: email, password: password);

      // Сохраняем токены
      final accessToken = response['access_token'] as String;
      final refreshToken = response['refresh_token'] as String;
      await TokenStorage.saveRefreshToken(refreshToken);

      // Устанавливаем токен в provider
      await ref.read(accessTokenProvider.notifier).setToken(accessToken);

      // Проверяем, что токен установлен
      final tokenCheck = ref.read(accessTokenProvider);
      if (tokenCheck == null || tokenCheck != accessToken) {
        throw Exception('Failed to set access token');
      }

      // Инвалидируем DioClient и AuthRepository чтобы они пересоздались с новым токеном
      ref.invalidate(dioClientProvider);
      ref.invalidate(authRepositoryProvider);

      // Получаем новый repository с обновленным DioClient
      final updatedRepository = ref.read(authRepositoryProvider);

      // Получаем данные пользователя (используем новый DioClient с токеном)
      final user = await updatedRepository.getCurrentUser();
      state = AsyncValue.data(user);
    } catch (e, stackTrace) {
      state = AsyncValue.error(e, stackTrace);
      rethrow;
    }
  }

  /// Выход пользователя
  Future<void> logout() async {
    try {
      final repository = ref.read(authRepositoryProvider);
      await repository.logout();
    } catch (e) {
      // Игнорируем ошибки при logout на сервере
      // Все равно очищаем локальные токены
    } finally {
      // Удаляем токены из secure storage
      await ref.read(accessTokenProvider.notifier).clear();

      // Инвалидируем DioClient и AuthRepository чтобы они пересоздались без токена
      ref.invalidate(dioClientProvider);
      ref.invalidate(authRepositoryProvider);

      // Очищаем состояние пользователя
      state = const AsyncValue.data(null);
    }
  }

  /// Обновить данные текущего пользователя
  Future<void> refreshUser() async {
    try {
      final repository = ref.read(authRepositoryProvider);
      final user = await repository.getCurrentUser();
      state = AsyncValue.data(user);
    } catch (e) {
      // Если токен невалидный, очищаем storage
      await ref.read(accessTokenProvider.notifier).clear();
      state = const AsyncValue.data(null);
    }
  }
}
