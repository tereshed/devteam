import 'package:dio/dio.dart';
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

/// Sprint 15.M6: CancelToken+ref.onDispose — при быстрой смене вкладок прерываем висящий
/// HTTP-запрос вместо ожидания timeout.
@riverpod
Future<List<LLMProviderModel>> llmProvidersList(Ref ref) async {
  final repo = ref.watch(llmProvidersRepositoryProvider);
  final cancelToken = CancelToken();
  ref.onDispose(() => cancelToken.cancel('llmProvidersList provider disposed'));
  return repo.list(cancelToken: cancelToken);
}
