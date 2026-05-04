import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/auth/data/auth_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'auth_providers.g.dart';

/// Provider для AuthRepository
///
/// Предоставляет экземпляр AuthRepository с настроенным Dio клиентом.
@Riverpod(keepAlive: true)
AuthRepository authRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return AuthRepository(dio: dio);
}
