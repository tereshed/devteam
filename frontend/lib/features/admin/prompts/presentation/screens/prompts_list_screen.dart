import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/prompts/data/prompts_providers.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_model.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

class PromptsListScreen extends ConsumerWidget {
  const PromptsListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final promptsAsync = ref.watch(promptsListProvider);
    final l10n = AppLocalizations.of(context)!;

    return Scaffold(
      appBar: AppBar(title: Text(l10n.promptsTitle)),
      body: promptsAsync.when(
        data: (prompts) => ListView.builder(
          itemCount: prompts.length,
          itemBuilder: (context, index) {
            final prompt = prompts[index];
            return PromptListTile(prompt: prompt);
          },
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, stack) =>
            Center(child: Text('${l10n.dataLoadError}: $err')),
      ),
    );
  }
}

class PromptListTile extends StatelessWidget {
  final Prompt prompt;

  const PromptListTile({super.key, required this.prompt});

  @override
  Widget build(BuildContext context) {
    return ListTile(
      title: Text(prompt.name),
      subtitle: Text(prompt.description),
      trailing: Icon(
        prompt.isActive ? Icons.check_circle : Icons.cancel,
        color: prompt.isActive ? Colors.green : Colors.grey,
      ),
      onTap: () => context.go('/admin/prompts/${prompt.id}'),
    );
  }
}
