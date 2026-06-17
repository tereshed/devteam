import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/sandbox_services/domain/models/sandbox_service_model.dart';
import 'package:frontend/features/sandbox_services/domain/sandbox_service_exceptions.dart';
import 'package:frontend/features/sandbox_services/presentation/controllers/sandbox_services_controller.dart';
import 'package:frontend/features/sandbox_services/presentation/widgets/sandbox_service_form_dialog.dart';

/// Вкладка «Тестовое окружение» в настройках проекта: список эфемерных
/// сервис-сайдкаров (postgres для интеграционных тестов с БД).
class SandboxServicesTab extends ConsumerWidget {
  const SandboxServicesTab({super.key, required this.projectId});

  final String projectId;

  Future<void> _openForm(
    BuildContext context,
    WidgetRef ref, {
    SandboxServiceModel? existing,
  }) async {
    final l10n = requireAppLocalizations(context, where: 'SandboxServicesTab');
    final result = await showDialog<SandboxServiceFormResult>(
      context: context,
      builder: (_) => SandboxServiceFormDialog(existing: existing),
    );
    if (result == null || !context.mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref
          .read(sandboxServicesControllerProvider(projectId).notifier)
          .upsert(
            alias: result.alias,
            isEnabled: result.isEnabled,
            kind: result.kind,
            image: result.image,
            dbName: result.dbName,
            dbUser: result.dbUser,
            port: result.port,
            seedKind: result.seedKind,
            seedValue: result.seedValue,
            readyTimeoutSeconds: result.readyTimeoutSeconds,
          );
      messenger.showSnackBar(SnackBar(content: Text(l10n.sandboxServicesSavedSnack)));
    } on SandboxServiceException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  Future<void> _toggle(
    WidgetRef ref,
    SandboxServiceModel s,
    bool enabled,
  ) async {
    await ref
        .read(sandboxServicesControllerProvider(projectId).notifier)
        .upsert(
          alias: s.alias,
          isEnabled: enabled,
          kind: s.kind,
          image: s.image,
          dbName: s.dbName,
          dbUser: s.dbUser,
          port: s.port,
          seedKind: s.seedKind,
          seedValue: s.seedValue,
          readyTimeoutSeconds: s.readyTimeoutSeconds,
        );
  }

  Future<void> _delete(
    BuildContext context,
    WidgetRef ref,
    SandboxServiceModel s,
  ) async {
    final l10n = requireAppLocalizations(context, where: 'SandboxServicesTab');
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.sandboxServiceDeleteTitle),
        content: Text(l10n.sandboxServiceDeleteConfirm(s.alias)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.sandboxServiceCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.sandboxServiceDelete),
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
          .read(sandboxServicesControllerProvider(projectId).notifier)
          .delete(s.id);
      messenger.showSnackBar(SnackBar(content: Text(l10n.sandboxServicesDeletedSnack)));
    } on SandboxServiceException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'SandboxServicesTab');
    final theme = Theme.of(context);
    final asyncServices = ref.watch(sandboxServicesControllerProvider(projectId));

    return RefreshIndicator(
      onRefresh: () =>
          ref.read(sandboxServicesControllerProvider(projectId).notifier).refresh(),
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          Text(l10n.sandboxServicesHeading, style: theme.textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            l10n.sandboxServicesDescription,
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
              label: Text(l10n.sandboxServicesAddButton),
            ),
          ),
          const SizedBox(height: 12),
          asyncServices.when(
            data: (services) => services.isEmpty
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Text(
                      l10n.sandboxServicesEmpty,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  )
                : Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      for (final s in services)
                        Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: _SandboxServiceCard(
                            service: s,
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
            error: (e, _) => _ErrorBox(message: l10n.sandboxServicesLoadError),
          ),
        ],
      ),
    );
  }
}

class _SandboxServiceCard extends StatelessWidget {
  const _SandboxServiceCard({
    required this.service,
    required this.onToggle,
    required this.onEdit,
    required this.onDelete,
  });

  final SandboxServiceModel service;
  final ValueChanged<bool> onToggle;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
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
                    '${service.alias} · ${service.image}',
                    style: theme.textTheme.titleMedium,
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${service.kind} · ${service.dbUser}@${service.dbName}:${service.port} · seed=${service.seedKind}',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
            Switch(value: service.isEnabled, onChanged: onToggle),
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
