import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/prompts/data/prompts_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

class PromptDetailScreen extends ConsumerWidget {
  final String promptId;

  const PromptDetailScreen({super.key, required this.promptId});

  void _showFullScreenContent(
    BuildContext context,
    String title,
    String content,
  ) {
    showDialog(
      context: context,
      builder: (context) => Dialog(
        insetPadding: const EdgeInsets.all(16),
        child: Scaffold(
          appBar: AppBar(title: Text(title)),
          body: Padding(
            padding: const EdgeInsets.all(16),
            child: SelectableText(content),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final promptAsync = ref.watch(promptDetailProvider(promptId));

    return Scaffold(
      appBar: AppBar(title: Text(l10n.promptsTitle)),
      body: promptAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, stack) =>
            Center(child: Text('${l10n.dataLoadError}: $err')),
        data: (prompt) => SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildDetailItem(context, l10n.promptName, prompt.name),
              const SizedBox(height: 16),
              _buildDetailItem(
                context,
                l10n.promptDescription,
                prompt.description,
              ),
              const SizedBox(height: 16),
              _buildStatusItem(context, l10n.promptIsActive, prompt.isActive),
              const SizedBox(height: 16),
              _buildExpandableItem(
                context,
                l10n.promptTemplate,
                prompt.template,
                onExpand: () => _showFullScreenContent(
                  context,
                  l10n.promptTemplate,
                  prompt.template,
                ),
              ),
              const SizedBox(height: 16),
              if (prompt.jsonSchema != null)
                _buildExpandableItem(
                  context,
                  l10n.promptJsonSchema,
                  const JsonEncoder.withIndent('  ').convert(prompt.jsonSchema),
                  onExpand: () => _showFullScreenContent(
                    context,
                    l10n.promptJsonSchema,
                    const JsonEncoder.withIndent(
                      '  ',
                    ).convert(prompt.jsonSchema),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDetailItem(BuildContext context, String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: Theme.of(context).textTheme.labelMedium),
        const SizedBox(height: 4),
        SelectableText(value, style: Theme.of(context).textTheme.bodyLarge),
      ],
    );
  }

  Widget _buildStatusItem(BuildContext context, String label, bool isActive) {
    return Row(
      children: [
        Text(label, style: Theme.of(context).textTheme.labelMedium),
        const SizedBox(width: 8),
        Icon(
          isActive ? Icons.check_circle : Icons.cancel,
          color: isActive ? Colors.green : Colors.grey,
        ),
      ],
    );
  }

  Widget _buildExpandableItem(
    BuildContext context,
    String label,
    String content, {
    required VoidCallback onExpand,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label, style: Theme.of(context).textTheme.labelMedium),
            IconButton(
              icon: const Icon(Icons.fullscreen),
              onPressed: onExpand,
              tooltip: 'Expand',
            ),
          ],
        ),
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            border: Border.all(color: Colors.grey.shade300),
            borderRadius: BorderRadius.circular(8),
          ),
          child: Text(
            content,
            maxLines: 5,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(fontFamily: 'monospace'),
          ),
        ),
      ],
    );
  }
}
