import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/schedules/domain/models/scheduled_task_model.dart';
import 'package:frontend/features/schedules/presentation/controllers/schedules_controller.dart';
import 'package:frontend/features/schedules/presentation/widgets/schedule_form_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Экран вкладки «Расписание» проекта: список регулярных задач + CRUD.
class SchedulesListScreen extends ConsumerWidget {
  const SchedulesListScreen({super.key, required this.projectId});

  final String projectId;

  Future<void> _openForm(
    BuildContext context, {
    ScheduledTaskModel? existing,
  }) async {
    await showDialog<bool>(
      context: context,
      builder: (_) => ScheduleFormDialog(projectId: projectId, existing: existing),
    );
  }

  Future<void> _confirmDelete(
    BuildContext context,
    WidgetRef ref,
    ScheduledTaskModel schedule,
  ) async {
    final l10n = requireAppLocalizations(context, where: 'SchedulesListScreen');
    final messenger = ScaffoldMessenger.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.scheduleDeleteTitle),
        content: Text(l10n.scheduleDeleteMessage),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.scheduleCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.scheduleDelete),
          ),
        ],
      ),
    );
    if (confirmed != true) {
      return;
    }
    try {
      await ref
          .read(schedulesControllerProvider(projectId).notifier)
          .deleteSchedule(schedule.id);
      messenger.showSnackBar(SnackBar(content: Text(l10n.scheduleDeletedSnack)));
    } catch (e) {
      messenger.showSnackBar(SnackBar(content: Text('$e')));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'SchedulesListScreen');
    final state = ref.watch(schedulesControllerProvider(projectId));

    return Scaffold(
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _openForm(context),
        icon: const Icon(Icons.add),
        label: Text(l10n.schedulesAdd),
      ),
      body: state.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => _ErrorView(
          message: l10n.schedulesLoadError,
          onRetry: () => ref.invalidate(schedulesControllerProvider(projectId)),
        ),
        data: (items) {
          if (items.isEmpty) {
            return _EmptyView(
              message: l10n.schedulesEmpty,
              actionLabel: l10n.schedulesAdd,
              onAction: () => _openForm(context),
            );
          }
          return RefreshIndicator(
            onRefresh: () async =>
                ref.invalidate(schedulesControllerProvider(projectId)),
            child: ListView.separated(
              padding: const EdgeInsets.all(16),
              itemCount: items.length,
              separatorBuilder: (_, _) => const SizedBox(height: 8),
              itemBuilder: (context, i) => _ScheduleCard(
                schedule: items[i],
                l10n: l10n,
                onEdit: () => _openForm(context, existing: items[i]),
                onDelete: () => _confirmDelete(context, ref, items[i]),
                onToggle: () => ref
                    .read(schedulesControllerProvider(projectId).notifier)
                    .toggleActive(items[i]),
              ),
            ),
          );
        },
      ),
    );
  }
}

class _ScheduleCard extends StatelessWidget {
  const _ScheduleCard({
    required this.schedule,
    required this.l10n,
    required this.onEdit,
    required this.onDelete,
    required this.onToggle,
  });

  final ScheduledTaskModel schedule;
  final AppLocalizations l10n;
  final VoidCallback onEdit;
  final VoidCallback onDelete;
  final VoidCallback onToggle;

  String _fmt(DateTime? dt) {
    if (dt == null) {
      return l10n.scheduleNeverRun;
    }
    final l = dt.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${l.year}-${two(l.month)}-${two(l.day)} ${two(l.hour)}:${two(l.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    schedule.name,
                    style: theme.textTheme.titleMedium,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                Chip(
                  label: Text(
                    schedule.isActive ? l10n.scheduleActive : l10n.scheduleInactive,
                  ),
                  backgroundColor: schedule.isActive
                      ? theme.colorScheme.secondaryContainer
                      : theme.colorScheme.surfaceContainerHighest,
                ),
              ],
            ),
            if (schedule.description.isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                schedule.description,
                style: theme.textTheme.bodyMedium,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ],
            const SizedBox(height: 8),
            Row(
              children: [
                const Icon(Icons.schedule, size: 16),
                const SizedBox(width: 6),
                Text(
                  schedule.cronExpression,
                  style: theme.textTheme.bodyMedium
                      ?.copyWith(fontFamily: 'monospace'),
                ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              '${l10n.scheduleNextRunLabel}: ${_fmt(schedule.nextRunAt)}',
              style: theme.textTheme.bodySmall,
            ),
            Text(
              '${l10n.scheduleLastRunLabel}: ${_fmt(schedule.lastRunAt)}',
              style: theme.textTheme.bodySmall,
            ),
            const SizedBox(height: 4),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                IconButton(
                  tooltip: schedule.isActive
                      ? l10n.scheduleDisableTooltip
                      : l10n.scheduleEnableTooltip,
                  icon: Icon(
                    schedule.isActive ? Icons.pause_circle : Icons.play_circle,
                  ),
                  onPressed: onToggle,
                ),
                IconButton(
                  tooltip: l10n.scheduleEdit,
                  icon: const Icon(Icons.edit_outlined),
                  onPressed: onEdit,
                ),
                IconButton(
                  tooltip: l10n.scheduleDelete,
                  icon: const Icon(Icons.delete_outline),
                  onPressed: onDelete,
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _EmptyView extends StatelessWidget {
  const _EmptyView({
    required this.message,
    required this.actionLabel,
    required this.onAction,
  });

  final String message;
  final String actionLabel;
  final VoidCallback onAction;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.event_repeat,
            size: 64,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text(message, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: onAction,
            icon: const Icon(Icons.add),
            label: Text(actionLabel),
          ),
        ],
      ),
    );
  }
}

class _ErrorView extends StatelessWidget {
  const _ErrorView({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.error_outline,
            size: 48,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 12),
          Text(message),
          const SizedBox(height: 12),
          OutlinedButton(onPressed: onRetry, child: const Text('↻')),
        ],
      ),
    );
  }
}
