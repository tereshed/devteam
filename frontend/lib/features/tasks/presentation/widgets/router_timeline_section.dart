import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:intl/intl.dart';

/// Лента решений Router'а по задаче — для Task Detail v2.
///
/// Каждая строка отображает шаг, выбранных агентов, причину и outcome (если DONE).
class RouterTimelineSection extends ConsumerWidget {
  const RouterTimelineSection({super.key, required this.taskId});

  final String taskId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'router_timeline_section');
    final async = ref.watch(taskRouterDecisionsProvider(taskId));
    return async.when(
      loading: () => const Padding(
        padding: EdgeInsets.all(8),
        child: LinearProgressIndicator(),
      ),
      error: (err, _) =>
          Text('${l10n.dataLoadError}: $err', style: const TextStyle(color: Colors.red)),
      data: (decisions) {
        if (decisions.isEmpty) {
          return Text(l10n.routerTimelineEmpty,
              style: Theme.of(context).textTheme.bodySmall);
        }
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children:
              decisions.map((d) => _DecisionTile(decision: d)).toList(),
        );
      },
    );
  }
}

class _DecisionTile extends StatelessWidget {
  const _DecisionTile({required this.decision});

  final RouterDecision decision;

  @override
  Widget build(BuildContext context) {
    final time =
        DateFormat.Hm().format(decision.createdAt.toLocal());
    final agentsLabel = decision.chosenAgents.isEmpty
        ? '—'
        : decision.chosenAgents.join(' · ');
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                CircleAvatar(
                  radius: 12,
                  backgroundColor: decision.done
                      ? Colors.green.shade100
                      : Theme.of(context).colorScheme.primary.withValues(alpha: 0.15),
                  child: Text(
                    '${decision.stepNo}',
                    style: const TextStyle(
                        fontSize: 11, fontWeight: FontWeight.w600),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    agentsLabel,
                    style: const TextStyle(
                        fontFamily: 'monospace', fontWeight: FontWeight.w600),
                  ),
                ),
                if (decision.outcome != null)
                  Chip(
                    label: Text(decision.outcome!),
                    visualDensity: VisualDensity.compact,
                  ),
                const SizedBox(width: 4),
                Text(time,
                    style: Theme.of(context).textTheme.bodySmall),
              ],
            ),
            const SizedBox(height: 6),
            Text(decision.reason,
                style: Theme.of(context).textTheme.bodySmall),
          ],
        ),
      ),
    );
  }
}
