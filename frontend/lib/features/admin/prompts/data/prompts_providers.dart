import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:frontend/features/admin/prompts/data/prompts_repository.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_model.dart';

final promptsRepositoryProvider = Provider<PromptsRepository>((ref) {
  final dio = ref.watch(dioClientProvider);
  return PromptsRepository(dio: dio);
});

final promptsListProvider = FutureProvider.autoDispose<List<Prompt>>((
  ref,
) async {
  final repository = ref.watch(promptsRepositoryProvider);
  return repository.getPrompts();
});

final promptDetailProvider = FutureProvider.autoDispose.family<Prompt, String>((
  ref,
  id,
) async {
  final repository = ref.watch(promptsRepositoryProvider);
  return repository.getPrompt(id);
});
