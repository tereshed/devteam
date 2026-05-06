import 'package:flutter/material.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_visuals.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Максимальная ширина карточки статуса задачи (см. ТЗ 11.7, без [LayoutBuilder]).
/// Узкий родитель ([ConstrainedBox] в ленте) сам ограничит фактическую ширину.
const double kTaskStatusCardMaxWidth = 400;

const BorderRadius _kCardBorderRadius = BorderRadius.all(Radius.circular(12));
const RoundedRectangleBorder _kCardShape =
    RoundedRectangleBorder(borderRadius: _kCardBorderRadius);

String _shortIdForDisplay(String taskId) {
  if (taskId.length <= 8) {
    return taskId;
  }
  return taskId.substring(0, 8);
}

/// Карточка статуса одной задачи в ленте чата (ТЗ 11.7).
class TaskStatusCard extends StatelessWidget {
  const TaskStatusCard({
    super.key,
    required this.taskId,
    this.title,
    required this.status,
    this.errorMessage,
    this.agentRole,
    this.onOpen,
  });

  final String taskId;
  final String? title;
  final String status;
  final String? errorMessage;
  final TaskCardAgentRole? agentRole;
  final ValueChanged<String>? onOpen;

  @override
  Widget build(BuildContext context) {
    assert(() {
      final k = key;
      if (k != null) {
        assert(k is ValueKey<String>, 'TaskStatusCard: key must be ValueKey<String>(taskId)');
        if (k is ValueKey<String>) {
          assert(k.value == taskId, 'TaskStatusCard: key.value must equal taskId');
        }
      }
      return true;
    }());

    if (taskId.isEmpty) {
      assert(() {
        debugPrint('TaskStatusCard: empty taskId');
        return true;
      }());
      return const SizedBox.shrink();
    }

    if (status.isEmpty) {
      assert(() {
        debugPrint('TaskStatusCard: empty status');
        return true;
      }());
    }

    assert(() {
      if (errorMessage != null && status.isNotEmpty && status != 'failed') {
        throw FlutterError(
          'TaskStatusCard: errorMessage is only allowed when status == failed',
        );
      }
      return true;
    }());

    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    final t = title?.trim();
    final displayTitle =
        (t != null && t.isNotEmpty) ? t : l10n.taskStatusCardFallbackTitle(_shortIdForDisplay(taskId));

    final statusLabel = taskStatusLabel(l10n, status);
    final cat = taskStatusVisualCategory(status);
    final iconBg = taskStatusContainerColor(scheme, cat);
    final iconFg = taskStatusOnContainerColor(scheme, cat);
    final icon = taskStatusIcon(cat);

    final roleText =
        agentRole != null ? taskCardAgentRoleLabel(l10n, agentRole!) : null;
    final statusLine = roleText != null
        ? '$statusLabel · $roleText'
        : statusLabel;

    final sem = StringBuffer(displayTitle)
      ..write(', ')
      ..write(statusLabel);
    if (roleText != null) {
      sem
        ..write(', ')
        ..write(roleText);
    }
    final semLabel = sem.toString();

    final body = Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          DecoratedBox(
            decoration: BoxDecoration(
              color: iconBg,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Icon(icon, color: iconFg, size: 22),
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  displayTitle,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: scheme.onSurface,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  statusLine,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
                if (status == 'failed' &&
                    (errorMessage?.trim().isNotEmpty ?? false)) ...[
                  const SizedBox(height: 8),
                  ExcludeSemantics(
                    child: Text(
                      errorMessage!.trim(),
                      maxLines: 3,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: scheme.error,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );

    final open = onOpen;
    final materialChild = open != null
        ? InkWell(
            onTap: () => open(taskId),
            borderRadius: _kCardBorderRadius,
            child: body,
          )
        : body;

    return Semantics(
      button: open != null,
      label: semLabel,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: kTaskStatusCardMaxWidth),
        child: Material(
          color: scheme.surfaceContainerLow,
          elevation: 0,
          shape: _kCardShape,
          clipBehavior: Clip.antiAlias,
          child: materialChild,
        ),
      ),
    );
  }
}
