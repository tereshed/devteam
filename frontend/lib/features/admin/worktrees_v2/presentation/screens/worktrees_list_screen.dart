import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/worktrees_v2/data/worktrees_providers.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';
import 'package:frontend/core/l10n/require.dart';
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
              itemBuilder: (_, i) => _WorktreeTile(worktree: items[i]),
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

class _WorktreeTile extends StatelessWidget {
  const _WorktreeTile({required this.worktree});

  final WorktreeV2 worktree;

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
      trailing: Chip(
        label: Text(worktree.state),
        backgroundColor: _stateColor(context).withValues(alpha: 0.12),
        side: BorderSide(color: _stateColor(context).withValues(alpha: 0.3)),
      ),
    );
  }
}
