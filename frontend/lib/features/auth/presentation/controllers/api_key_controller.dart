import 'package:frontend/features/auth/data/api_key_providers.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'api_key_controller.g.dart';

/// ApiKeyController управляет состоянием API-ключей пользователя
@riverpod
class ApiKeyController extends _$ApiKeyController {
  @override
  Future<List<ApiKeyModel>> build() async {
    final repository = ref.read(apiKeyRepositoryProvider);
    return repository.listKeys();
  }

  /// Создание нового API-ключа
  /// Возвращает ApiKeyCreatedModel с сырым ключом (показывается один раз)
  Future<ApiKeyCreatedModel> createKey({
    required String name,
    String? scopes,
    int? expiresInSeconds,
  }) async {
    final repository = ref.read(apiKeyRepositoryProvider);
    final created = await repository.createKey(
      name: name,
      scopes: scopes,
      expiresInSeconds: expiresInSeconds,
    );

    // Обновляем список ключей
    ref.invalidateSelf();

    return created;
  }

  /// Отзыв API-ключа
  Future<void> revokeKey(String id) async {
    final repository = ref.read(apiKeyRepositoryProvider);
    await repository.revokeKey(id);

    // Обновляем список ключей
    ref.invalidateSelf();
  }

  /// Удаление API-ключа
  Future<void> deleteKey(String id) async {
    final repository = ref.read(apiKeyRepositoryProvider);
    await repository.deleteKey(id);

    // Обновляем список ключей
    ref.invalidateSelf();
  }
}
