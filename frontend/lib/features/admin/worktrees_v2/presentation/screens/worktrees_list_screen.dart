import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/admin/worktrees_v2/data/worktrees_providers.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktrees_exceptions.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:intl/intl.dart';

class WorktreesListScreen extends ConsumerWidget {
  const WorktreesListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'worktrees_list_screen');
    final async = ref.watch(worktreesListProvider);
    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.worktreesTitle),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: l10n.agentsV2Refresh,
            onPressed: () => ref.invalidate(worktreesListProvider),
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(48),
          child: _StateFilterBar(l10n: l10n),
        ),
      ),
      body: async.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) =>
            Center(child: Text('${l10n.dataLoadError}: $err')),
        data: (items) {
          if (items.isEmpty) {
            return Center(child: Text(l10n.worktreesEmpty));
          }
          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(worktreesListProvider);
              await ref.read(worktreesListProvider.future);
            },
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: items.length,
              separatorBuilder: (_, __) => const Divider(height: 0),
              itemBuilder: (_, i) => _WorktreeTile(
                worktree: items[i],
                onRelease: () => _confirmAndRelease(context, ref, items[i]),
              ),
            ),
          );
        },
      ),
    );
  }
}

class _StateFilterBar extends ConsumerWidget {
  const _StateFilterBar({required this.l10n});

  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(worktreesStateFilterProvider);
    final entries = <(String?, String)>[
      (null, l10n.worktreesFilterAll),
      ('allocated', l10n.worktreesFilterAllocated),
      ('in_use', l10n.worktreesFilterInUse),
      ('released', l10n.worktreesFilterReleased),
    ];
    return SizedBox(
      height: 48,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        itemCount: entries.length,
        separatorBuilder: (_, __) => const SizedBox(width: 8),
        itemBuilder: (_, i) {
          final entry = entries[i];
          final value = entry.$1;
          final isSelected = value == selected;
          return ChoiceChip(
            label: Text(entry.$2),
            selected: isSelected,
            onSelected: (chosen) {
              if (!chosen) return;
              ref.read(worktreesStateFilterProvider.notifier).set(value);
            },
          );
        },
      ),
    );
  }
}

/// Manual unstick flow: показывает confirm-диалог, дёргает repo.release, мапит
/// исключения в snackbar'ы. Вынесено наружу из _WorktreeTile, чтобы
/// (a) виджет остался Stateless, (b) тест мог дёрнуть только верхний уровень.
Future<void> _confirmAndRelease(
  BuildContext context,
  WidgetRef ref,
  WorktreeV2 wt,
) async {
  final l10n = requireAppLocalizations(context, where: 'worktrees_list_screen');
  final confirmed = await showDialog<bool>(
    context: context,
    builder: (ctx) => AlertDialog(
      title: Text(l10n.worktreesReleaseDialogTitle),
      content: Text(l10n.worktreesReleaseDialogBody),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(ctx).pop(false),
          child: Text(l10n.commonCancel),
        ),
        FilledButton.tonal(
          style: FilledButton.styleFrom(
            backgroundColor: Theme.of(ctx).colorScheme.errorContainer,
            foregroundColor: Theme.of(ctx).colorScheme.onErrorContainer,
          ),
          onPressed: () => Navigator.of(ctx).pop(true),
          child: Text(l10n.worktreesReleaseButton),
        ),
      ],
    ),
  );
  if (confirmed != true) {
    return;
  }
  // ignore: use_build_context_synchronously — гард mounted ниже.
  if (!context.mounted) {
    return;
  }

  final repo = ref.read(worktreesRepositoryProvider);
  final messenger = ScaffoldMessenger.of(context);
  try {
    await repo.release(wt.id);
    if (!context.mounted) {
      return;
    }
    messenger.showSnackBar(
      SnackBar(content: Text(l10n.worktreesReleasedSnackbar)),
    );
  } on WorktreesConflictException {
    // Backend сообщил что worktree уже released — info-toast, не red error.
    if (!context.mounted) {
      return;
    }
    messenger.showSnackBar(
      SnackBar(content: Text(l10n.worktreesReleaseAlreadyReleased)),
    );
  } on WorktreesNotConfiguredException {
    // 503 + feature_not_configured: фича отключена на сервере. Сообщение
    // конкретное (root cause виден сразу), без $e — внутренние детали типа
    // env-имён уже в самом тексте l10n-строки.
    if (!context.mounted) {
      return;
    }
    messenger.showSnackBar(
      SnackBar(content: Text(l10n.worktreesReleaseNotConfigured)),
    );
  } catch (e) {
    if (!context.mounted) {
      return;
    }
    messenger.showSnackBar(
      SnackBar(content: Text('${l10n.worktreesReleaseFailed}: $e')),
    );
  } finally {
    // Инвалидация даже на ошибке: backend мог успеть пометить released перед
    // network-failure'ом, а stale-tile введёт оператора в заблуждение.
    ref.invalidate(worktreesListProvider);
  }
}

class _WorktreeTile extends StatelessWidget {
  const _WorktreeTile({
    required this.worktree,
    required this.onRelease,
  });

  final WorktreeV2 worktree;
  final VoidCallback onRelease;

  Color _stateColor(BuildContext context) {
    switch (worktree.state) {
      case 'allocated':
        return Colors.amber;
      case 'in_use':
        return Colors.blue;
      case 'released':
        return Colors.grey;
      default:
        return Theme.of(context).colorScheme.outline;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'worktrees_list_screen');
    final allocated =
        DateFormat.yMMMd().add_Hm().format(worktree.allocatedAt.toLocal());
    return ListTile(
      leading: CircleAvatar(
        backgroundColor: _stateColor(context).withValues(alpha: 0.15),
        child: Icon(Icons.account_tree, color: _stateColor(context)),
      ),
      title: Text(
        worktree.branchName,
        style: const TextStyle(fontFamily: 'monospace'),
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '${l10n.worktreesColTask}: ${worktree.taskId}',
            style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
          ),
          Text(
            'base: ${worktree.baseBranch} · ${l10n.worktreesColAllocated}: $allocated',
            style: const TextStyle(fontSize: 12),
          ),
        ],
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Chip(
            label: Text(worktree.state),
            backgroundColor: _stateColor(context).withValues(alpha: 0.12),
            side: BorderSide(color: _stateColor(context).withValues(alpha: 0.3)),
          ),
          // Кнопка показывается только для не-released worktree'ев — для уже
          // released'ов release ничего не сделает (backend вернёт 409), смысла
          // её рендерить нет.
          if (worktree.state != 'released')
            IconButton(
              icon: const Icon(Icons.cleaning_services_outlined),
              tooltip: l10n.worktreesReleaseButton,
              onPressed: onRelease,
            ),
        ],
      ),
    );
  }
}
