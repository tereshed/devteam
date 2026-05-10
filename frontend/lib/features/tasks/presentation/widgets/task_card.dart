import 'package:flutter/material.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/tasks/domain/models/task_list_item_model.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:intl/intl.dart';

/// Карточка строки задачи: статус, приоритет, агент, время обновления (12.6).
class TaskCard extends StatelessWidget {
  const TaskCard({
    super.key,
    required this.task,
    this.onTap,
    this.dense = false,
  });

  final TaskListItemModel task;
  final VoidCallback? onTap;
  final bool dense;

  String _formatUpdatedAt(BuildContext context) {
    final localeTag = Localizations.localeOf(context).toLanguageTag();
    final at = task.updatedAt.toLocal();
    final fmt = dense
        ? DateFormat.MMMd(localeTag).add_Hm()
        : DateFormat.yMMMd(localeTag).add_jm();
    return fmt.format(at);
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    final tt = Theme.of(context).textTheme;
    final bodyMutedSmall = tt.bodySmall?.copyWith(color: scheme.onSurfaceVariant);
    final stTone = taskStatusTone(task.status);
    final prTone = taskPriorityTone(task.priority);

    final content = Padding(
      padding: EdgeInsets.all(dense ? 10 : 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ExcludeSemantics(
            excluding: onTap != null,
            child: Text(
              task.title,
              style: tt.titleSmall,
              maxLines: dense ? 3 : 4,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          SizedBox(height: dense ? 6 : 10),
          Wrap(
            spacing: 8,
            runSpacing: 6,
            children: [
              _TaskCardChip(
                icon: taskStatusIcon(stTone),
                label: taskStatusLabel(l10n, task.status),
                foreground: taskStatusChipForeground(scheme, stTone),
                background: taskStatusChipBackground(scheme, stTone),
              ),
              _TaskCardChip(
                icon: taskPriorityIcon(prTone),
                label: taskPriorityLabel(l10n, task.priority),
                foreground: taskPriorityChipForeground(scheme, prTone),
                background: taskPriorityChipBackground(scheme, prTone),
              ),
            ],
          ),
          SizedBox(height: dense ? 6 : 8),
          Text(
            task.assignedAgent != null
                ? l10n.taskCardAgentLine(
                    task.assignedAgent!.name,
                    agentRoleLabel(l10n, task.assignedAgent!.role),
                  )
                : l10n.taskCardUnassigned,
            style: bodyMutedSmall,
          ),
          SizedBox(height: dense ? 4 : 6),
          Text(
            l10n.taskCardUpdatedAt(_formatUpdatedAt(context)),
            style: bodyMutedSmall,
          ),
        ],
      ),
    );

    final material = Material(
      color: scheme.surfaceContainerLow,
      borderRadius: BorderRadius.circular(12),
      child: onTap != null
          ? InkWell(
              borderRadius: BorderRadius.circular(12),
              onTap: onTap,
              child: content,
            )
          : content,
    );

    if (onTap != null) {
      return Semantics(
        container: true,
        button: true,
        label: task.title,
        child: material,
      );
    }
    return material;
  }
}

class _TaskCardChip extends StatelessWidget {
  const _TaskCardChip({
    required this.icon,
    required this.label,
    required this.foreground,
    required this.background,
  });

  final IconData icon;
  final String label;
  final Color foreground;
  final Color background;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          ExcludeSemantics(
            child: Icon(icon, size: 14, color: foreground),
          ),
          const SizedBox(width: 4),
          Text(
            label,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  color: foreground,
                ),
          ),
        ],
      ),
    );
  }
}
