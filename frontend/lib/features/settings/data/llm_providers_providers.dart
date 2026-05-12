import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/settings/data/llm_providers_repository.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'llm_providers_providers.g.dart';

/// Sprint 15.29 — Riverpod-провайдеры для LLMProvidersRepository + AsyncList.
@Riverpod(keepAlive: true)
LLMProvidersRepository llmProvidersRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return LLMProvidersRepository(dio: dio);
}

@riverpod
Future<List<LLMProviderModel>> llmProvidersList(Ref ref) async {
  final repo = ref.watch(llmProvidersRepositoryProvider);
  return repo.list();
}
