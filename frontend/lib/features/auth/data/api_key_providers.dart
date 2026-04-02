import 'package:frontend/features/auth/data/api_key_repository.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'api_key_providers.g.dart';

/// Provider для ApiKeyRepository
@Riverpod(keepAlive: true)
ApiKeyRepository apiKeyRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ApiKeyRepository(dio: dio);
}
