import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/assistant/data/assistant_repository.dart';
import 'package:frontend/features/assistant/domain/assistant_status_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_providers.g.dart';

/// Один экземпляр на приложение (singleton-репозиторий). Параллель
/// `conversationRepositoryProvider`.
@Riverpod(keepAlive: true)
AssistantRepository assistantRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return AssistantRepository(dio: dio);
}

@riverpod
Future<AssistantStatusModel> assistantStatus(Ref ref) {
  return ref.watch(assistantRepositoryProvider).getStatus();
}
