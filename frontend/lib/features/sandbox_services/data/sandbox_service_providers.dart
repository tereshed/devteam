import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/sandbox_services/data/sandbox_service_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sandbox_service_providers.g.dart';

@Riverpod(keepAlive: true)
SandboxServiceRepository sandboxServiceRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return SandboxServiceRepository(dio: dio);
}
