import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/core/l10n/require.dart';

/// DAG-секция артефактов задачи (Task Detail v2):
///   • Все артефакты группируются по kind.
///   • subtask_description'ы отображаются с цепочкой depends_on (стрелка → id).
///
/// Это не canvas-визуализация графа, а компактный иерархический список,
/// который читается без панорамы/зума и работает на любом размере экрана.
/// При необходимости можно заменить на flutter_force_directed_graph в будущем.
class ArtifactsDagSection extends ConsumerWidget {
  const ArtifactsDagSection({super.key, required this.taskId});

  final String taskId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'artifacts_dag_section');
    final async = ref.watch(taskArtifactsProvider(taskId));
    return async.when(
      loading: () => const Padding(
        padding: EdgeInsets.all(8),
        child: LinearProgressIndicator(),
      ),
      error: (err, _) => Text('${l10n.dataLoadError}: $err',
          style: const TextStyle(color: Colors.red)),
      data: (items) {
        if (items.isEmpty) {
          return Text(l10n.artifactsEmpty,
              style: Theme.of(context).textTheme.bodySmall);
        }
        // Группируем по kind. Subtask'и идут вперёд (DAG акцент).
        final groups = <String, List<Artifact>>{};
        for (final a in items) {
          groups.putIfAbsent(a.kind, () => []).add(a);
        }
        const order = [
          'plan',
          'subtask_description',
          'code_diff',
          'review',
          'merged_code',
          'test_result',
        ];
        final orderedKinds = [
          ...order.where(groups.containsKey),
          ...groups.keys.where((k) => !order.contains(k)),
        ];
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            for (final kind in orderedKinds) ...[
              Padding(
                padding: const EdgeInsets.only(top: 8, bottom: 4),
                child: Text(kind,
                    style: const TextStyle(
                      fontFamily: 'monospace',
                      fontWeight: FontWeight.w600,
                    )),
              ),
              ...groups[kind]!.map((a) => _ArtifactTile(artifact: a)),
            ],
          ],
        );
      },
    );
  }
}

class _ArtifactTile extends StatelessWidget {
  const _ArtifactTile({required this.artifact});

  final Artifact artifact;

  String _shortId(String id) =>
      id.length > 8 ? '${id.substring(0, 8)}…' : id;

  @override
  Widget build(BuildContext context) {
    final title = artifact.subtaskTitle ?? artifact.summary;
    final isSubtask = artifact.kind == 'subtask_description';
    final depsLabel = isSubtask && artifact.dependsOn.isNotEmpty
        ? '← ${artifact.dependsOn.map(_shortId).join(', ')}'
        : null;
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 3),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _statusDot(artifact.status),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    title.isEmpty ? '(no summary)' : title,
                    style: const TextStyle(fontWeight: FontWeight.w500),
                  ),
                ),
                Text(
                  '${artifact.producerAgent} · #${artifact.iteration}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
            if (depsLabel != null) ...[
              const SizedBox(height: 4),
              Text(
                depsLabel,
                style: const TextStyle(
                    fontFamily: 'monospace', fontSize: 12, color: Colors.blueGrey),
              ),
            ],
            if (artifact.subtaskTitle != null &&
                artifact.summary.isNotEmpty &&
                artifact.summary != artifact.subtaskTitle) ...[
              const SizedBox(height: 4),
              Text(
                artifact.summary,
                style: Theme.of(context).textTheme.bodySmall,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _statusDot(String status) {
    final color = status == 'superseded' ? Colors.grey : Colors.green;
    return Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
    );
  }
}
