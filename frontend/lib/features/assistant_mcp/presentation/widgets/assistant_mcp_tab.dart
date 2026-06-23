import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant_mcp/domain/assistant_mcp_exceptions.dart';
import 'package:frontend/features/assistant_mcp/domain/models/assistant_mcp_server_model.dart';
import 'package:frontend/features/assistant_mcp/presentation/controllers/assistant_mcp_controller.dart';
import 'package:frontend/features/assistant_mcp/presentation/widgets/assistant_mcp_form_dialog.dart';

/// Вкладка «MCP-серверы» в настройках проекта: внешние MCP-серверы (remote
/// http/sse), инструменты которых доступны ассистенту проекта.
class AssistantMcpTab extends ConsumerWidget {
  const AssistantMcpTab({super.key, required this.projectId});

  final String projectId;

  Future<void> _openForm(
    BuildContext context,
    WidgetRef ref, {
    AssistantMcpServerModel? existing,
  }) async {
    final l10n = requireAppLocalizations(context, where: 'AssistantMcpTab');
    final result = await showDialog<AssistantMcpFormResult>(
      context: context,
      builder: (_) => AssistantMcpFormDialog(existing: existing),
    );
    if (result == null || !context.mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    final controller =
        ref.read(assistantMcpControllerProvider(projectId).notifier);
    try {
      if (existing == null) {
        await controller.create(
          name: result.name,
          transport: result.transport,
          url: result.url,
          headers: result.headers,
          requireConfirmation: result.requireConfirmation,
          isEnabled: result.isEnabled,
        );
      } else {
        await controller.updateServer(
          existing.id,
          name: result.name,
          transport: result.transport,
          url: result.url,
          headers: result.headers,
          requireConfirmation: result.requireConfirmation,
          isEnabled: result.isEnabled,
        );
      }
      messenger
          .showSnackBar(SnackBar(content: Text(l10n.assistantMcpSavedSnack)));
    } on AssistantMcpException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _toggle(
    WidgetRef ref,
    AssistantMcpServerModel s,
    bool enabled,
  ) async {
    await ref
        .read(assistantMcpControllerProvider(projectId).notifier)
        .toggleEnabled(s, enabled);
  }

  Future<void> _delete(
    BuildContext context,
    WidgetRef ref,
    AssistantMcpServerModel s,
  ) async {
    final l10n = requireAppLocalizations(context, where: 'AssistantMcpTab');
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.assistantMcpDeleteTitle),
        content: Text(l10n.assistantMcpDeleteConfirm(s.name)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.assistantMcpCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.assistantMcpDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !context.mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(assistantMcpControllerProvider(projectId).notifier)
          .delete(s.id);
      messenger
          .showSnackBar(SnackBar(content: Text(l10n.assistantMcpDeletedSnack)));
    } on AssistantMcpException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AssistantMcpTab');
    final theme = Theme.of(context);
    final asyncServers = ref.watch(assistantMcpControllerProvider(projectId));

    return RefreshIndicator(
      onRefresh: () => ref
          .read(assistantMcpControllerProvider(projectId).notifier)
          .refresh(),
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          Text(l10n.assistantMcpHeading, style: theme.textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            l10n.assistantMcpDescription,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          Align(
            alignment: Alignment.centerLeft,
            child: FilledButton.icon(
              onPressed: () => _openForm(context, ref),
              icon: const Icon(Icons.add),
              label: Text(l10n.assistantMcpAddButton),
            ),
          ),
          const SizedBox(height: 12),
          asyncServers.when(
            data: (servers) => servers.isEmpty
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Text(
                      l10n.assistantMcpEmpty,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  )
                : Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      for (final s in servers)
                        Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: _AssistantMcpCard(
                            server: s,
                            onToggle: (v) => _toggle(ref, s, v),
                            onEdit: () => _openForm(context, ref, existing: s),
                            onDelete: () => _delete(context, ref, s),
                          ),
                        ),
                    ],
                  ),
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => _ErrorBox(message: l10n.assistantMcpLoadError),
          ),
        ],
      ),
    );
  }
}

class _AssistantMcpCard extends StatelessWidget {
  const _AssistantMcpCard({
    required this.server,
    required this.onToggle,
    required this.onEdit,
    required this.onDelete,
  });

  final AssistantMcpServerModel server;
  final ValueChanged<bool> onToggle;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final l10n = requireAppLocalizations(context, where: 'AssistantMcpCard');
    final confirm = server.requireConfirmation
        ? ' · ${l10n.assistantMcpRequireConfirmationLabel}'
        : '';
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 8, 8, 8),
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '${server.name} · ${server.transport}$confirm',
                    style: theme.textTheme.titleMedium,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    server.url,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            Switch(value: server.isEnabled, onChanged: onToggle),
            IconButton(
              icon: const Icon(Icons.edit_outlined),
              onPressed: onEdit,
            ),
            IconButton(
              icon: const Icon(Icons.delete_outline),
              onPressed: onDelete,
            ),
          ],
        ),
      ),
    );
  }
}

class _ErrorBox extends StatelessWidget {
  const _ErrorBox({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.errorContainer,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        message,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: theme.colorScheme.onErrorContainer,
        ),
      ),
    );
  }
}
