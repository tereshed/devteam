import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/features/webhooks/data/webhook_providers.dart';
import 'package:frontend/features/webhooks/data/webhook_repository.dart';
import 'package:frontend/features/webhooks/domain/models/webhook_model.dart';
import 'package:frontend/features/webhooks/presentation/widgets/webhook_edit_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';

class WebhooksListSection extends ConsumerWidget {
  final String? projectId;

  const WebhooksListSection({super.key, this.projectId});

  Future<void> _openCreateDialog(BuildContext context, WidgetRef ref) async {
    final req = await showDialog<CreateWebhookRequest>(
      context: context,
      builder: (context) => WebhookEditDialog(projectId: projectId),
    );

    if (req != null) {
      try {
        final repo = ref.read(webhookRepositoryProvider);
        await repo.createWebhook(req);
        ref.invalidate(webhooksProvider);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Error: $e')),
          );
        }
      }
    }
  }

  Future<void> _openEditDialog(BuildContext context, WidgetRef ref, WebhookModel webhook) async {
    final req = await showDialog<UpdateWebhookRequest>(
      context: context,
      builder: (context) => WebhookEditDialog(
        projectId: projectId,
        webhook: webhook,
      ),
    );

    if (req != null) {
      try {
        final repo = ref.read(webhookRepositoryProvider);
        await repo.updateWebhook(webhook.id, req);
        ref.invalidate(webhooksProvider);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Error: $e')),
          );
        }
      }
    }
  }

  Future<void> _deleteWebhook(BuildContext context, WidgetRef ref, WebhookModel webhook) async {
    final l10n = AppLocalizations.of(context)!;
    final confirm = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(l10n.webhookDelete),
        content: Text(l10n.webhookDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(MaterialLocalizations.of(context).cancelButtonLabel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
              foregroundColor: Theme.of(context).colorScheme.onError,
            ),
            child: Text(l10n.webhookDelete),
          ),
        ],
      ),
    );

    if (confirm == true) {
      try {
        final repo = ref.read(webhookRepositoryProvider);
        await repo.deleteWebhook(webhook.id);
        ref.invalidate(webhooksProvider);
      } catch (e) {
        if (context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('Error: $e')),
          );
        }
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final provider = projectId != null
        ? projectWebhooksProvider(projectId!)
        : globalWebhooksProvider;

    final asyncWebhooks = ref.watch(provider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(l10n.webhooksTitle, style: theme.textTheme.titleMedium),
            FilledButton.icon(
              onPressed: () => _openCreateDialog(context, ref),
              icon: const Icon(Icons.add),
              label: Text(l10n.webhookCreate),
            ),
          ],
        ),
        SizedBox(height: Spacing.medium(context)),
        asyncWebhooks.when(
          data: (webhooks) {
            if (webhooks.isEmpty) {
              return Center(
                child: Padding(
                  padding: EdgeInsets.all(Spacing.large(context)),
                  child: Text(
                    l10n.webhooksEmpty,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurface.withValues(alpha: 0.6),
                    ),
                  ),
                ),
              );
            }
            return ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: webhooks.length,
              separatorBuilder: (context, index) => const Divider(),
              itemBuilder: (context, index) {
                final webhook = webhooks[index];
                return ListTile(
                  title: Text(webhook.name),
                  subtitle: Text(webhook.webhookUrl),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      IconButton(
                        icon: const Icon(Icons.edit),
                        onPressed: () => _openEditDialog(context, ref, webhook),
                      ),
                      IconButton(
                        icon: const Icon(Icons.delete, color: Colors.red),
                        onPressed: () => _deleteWebhook(context, ref, webhook),
                      ),
                    ],
                  ),
                );
              },
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Text(err.toString(), style: TextStyle(color: theme.colorScheme.error)),
        ),
      ],
    );
  }
}
