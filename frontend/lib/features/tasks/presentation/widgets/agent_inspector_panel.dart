import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifact_viewer_dialog.dart';
import 'package:frontend/features/tasks/presentation/widgets/sandbox_logs_viewer.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_execution_graph.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Компактный инспектор выбранного агента.
///
/// Вместо трёх вкладок во всю высоту (где пустая вкладка занимала всю панель) —
/// плотный единый скролл: шапка + мета-строка + секции, которые показываются
/// только если в них есть данные (артефакты, подзадачи), плюс сворачиваемые логи.
class AgentInspectorPanel extends ConsumerStatefulWidget {
  const AgentInspectorPanel({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.agent,
    required this.onClose,
  });

  final String projectId;
  final String taskId;
  final AgentNodeData agent;
  final VoidCallback onClose;

  @override
  ConsumerState<AgentInspectorPanel> createState() =>
      _AgentInspectorPanelState();
}

class _AgentInspectorPanelState extends ConsumerState<AgentInspectorPanel> {
  int _selectedSubtaskIndex = 0;

  @override
  void didUpdateWidget(covariant AgentInspectorPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.agent.id != widget.agent.id) {
      _selectedSubtaskIndex = 0;
    }
  }

  // Завершённые ноды — серые (без зелёного), живые — синие, ошибки — красные.
  Color get _statusColor => switch (widget.agent.status) {
    NodeStatus.pending => Colors.grey.shade600,
    NodeStatus.running => Colors.blue,
    NodeStatus.success => Colors.grey.shade400,
    NodeStatus.failed => Colors.red,
  };

  String _statusLabel(AppLocalizations l10n) => switch (widget.agent.status) {
    NodeStatus.pending => l10n.agentMatrixStatusPending,
    NodeStatus.running => l10n.agentMatrixStatusRunning,
    NodeStatus.success => l10n.agentMatrixStatusSuccess,
    NodeStatus.failed => l10n.agentMatrixStatusFailed,
  };

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'agent_inspector_panel',
    );
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final agent = widget.agent;
    final hasSubtasks = agent.subtasks.isNotEmpty;
    final hasArtifacts = agent.artifacts.isNotEmpty;
    final isRunning = agent.status == NodeStatus.running;

    return Container(
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: Border(left: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _header(theme, scheme, l10n),
          Expanded(
            child: ListView(
              padding: const EdgeInsets.only(bottom: 16),
              children: [
                _metaRow(theme, scheme, l10n),
                if (hasArtifacts) ...[
                  _sectionHeader(
                    theme,
                    scheme,
                    l10n.agentMatrixInspectorArtifacts,
                  ),
                  ...agent.artifacts.map((a) => _artifactRow(theme, scheme, a)),
                ],
                if (hasSubtasks) ...[
                  _sectionHeader(
                    theme,
                    scheme,
                    l10n.agentMatrixInspectorSubtasks,
                  ),
                  _subtasks(theme),
                ],
                if (!hasArtifacts && !hasSubtasks && !isRunning)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
                    child: Text(
                      l10n.agentMatrixInspectorNoArtifacts,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                _logsSection(theme, scheme, l10n, initiallyExpanded: isRunning),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _header(ThemeData theme, ColorScheme scheme, AppLocalizations l10n) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      color: scheme.surfaceContainerLow,
      child: Row(
        children: [
          CircleAvatar(
            backgroundColor: _statusColor.withValues(alpha: 0.12),
            radius: 14,
            child: Icon(
              _getRoleIcon(widget.agent.role),
              color: _statusColor,
              size: 16,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  widget.agent.name,
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                Text(
                  widget.agent.role.toUpperCase(),
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: scheme.outline,
                    fontWeight: FontWeight.w600,
                    letterSpacing: 0.6,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            onPressed: widget.onClose,
            icon: const Icon(Icons.close, size: 18),
            visualDensity: VisualDensity.compact,
            tooltip: l10n.commonCancel,
          ),
        ],
      ),
    );
  }

  Widget _metaRow(ThemeData theme, ColorScheme scheme, AppLocalizations l10n) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 12, 12, 4),
      child: Wrap(
        spacing: 8,
        runSpacing: 6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          _statusChip(),
          _miniChip(scheme, 'step ${widget.agent.stepNo}'),
          if (widget.agent.artifacts.isNotEmpty)
            _countChip(
              scheme,
              Icons.description_outlined,
              widget.agent.artifacts.length,
            ),
          if (widget.agent.subtasks.isNotEmpty)
            _countChip(
              scheme,
              Icons.checklist_rtl_outlined,
              widget.agent.subtasks.length,
            ),
        ],
      ),
    );
  }

  Widget _statusChip() {
    final c = _statusColor;
    final l10n = requireAppLocalizations(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 3),
      decoration: BoxDecoration(
        color: c.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(6),
        border: Border.all(color: c.withValues(alpha: 0.5)),
      ),
      child: Text(
        _statusLabel(l10n),
        style: TextStyle(color: c, fontSize: 11, fontWeight: FontWeight.w600),
      ),
    );
  }

  Widget _miniChip(ColorScheme scheme, String text) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
    decoration: BoxDecoration(
      color: scheme.surfaceContainerHighest.withValues(alpha: 0.5),
      borderRadius: BorderRadius.circular(6),
    ),
    child: Text(
      text,
      style: TextStyle(color: scheme.onSurfaceVariant, fontSize: 11),
    ),
  );

  Widget _countChip(ColorScheme scheme, IconData icon, int n) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
    decoration: BoxDecoration(
      color: scheme.surfaceContainerHighest.withValues(alpha: 0.5),
      borderRadius: BorderRadius.circular(6),
    ),
    child: Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 13, color: scheme.onSurfaceVariant),
        const SizedBox(width: 4),
        Text(
          '$n',
          style: TextStyle(color: scheme.onSurfaceVariant, fontSize: 11),
        ),
      ],
    ),
  );

  Widget _sectionHeader(ThemeData theme, ColorScheme scheme, String text) =>
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 6),
        child: Text(
          text.toUpperCase(),
          style: theme.textTheme.labelSmall?.copyWith(
            color: scheme.onSurfaceVariant,
            letterSpacing: 0.8,
            fontWeight: FontWeight.w700,
          ),
        ),
      );

  Widget _artifactRow(ThemeData theme, ColorScheme scheme, dynamic art) {
    return InkWell(
      onTap: () => showArtifactViewerDialog(
        context,
        taskId: widget.taskId,
        artifactId: art.id as String,
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        child: Row(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                borderRadius: BorderRadius.circular(5),
              ),
              child: Text(
                art.kind as String,
                style: const TextStyle(
                  fontSize: 10,
                  fontFamily: 'monospace',
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                (art.subtaskTitle as String?) ?? (art.summary as String),
                style: theme.textTheme.bodySmall,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            if ((art.iteration as int) > 0) ...[
              const SizedBox(width: 6),
              Text(
                '#${art.iteration}',
                style: TextStyle(fontSize: 10, color: scheme.onSurfaceVariant),
              ),
            ],
            const SizedBox(width: 4),
            Icon(Icons.open_in_new, size: 15, color: scheme.onSurfaceVariant),
          ],
        ),
      ),
    );
  }

  Widget _subtasks(ThemeData theme) {
    final subtasks = widget.agent.subtasks;
    final selected = subtasks[_selectedSubtaskIndex];

    var description = '';
    for (final art in widget.agent.artifacts) {
      if (art.kind == 'subtask_description' &&
          (art.subtaskTitle == selected || art.summary == selected)) {
        description = art.content?['description'] as String? ?? art.summary;
        break;
      }
    }
    if (description.isEmpty) {
      description = selected;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          height: 36,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 16),
            itemCount: subtasks.length,
            separatorBuilder: (_, _) => const SizedBox(width: 6),
            itemBuilder: (context, index) {
              final title = subtasks[index];
              final isSel = index == _selectedSubtaskIndex;
              return ChoiceChip(
                visualDensity: VisualDensity.compact,
                label: Text(
                  title.length > 20 ? '${title.substring(0, 18)}…' : title,
                  style: const TextStyle(fontSize: 11),
                ),
                selected: isSel,
                onSelected: (v) {
                  if (v) {
                    setState(() => _selectedSubtaskIndex = index);
                  }
                },
              );
            },
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: Text(
            description,
            style: theme.textTheme.bodySmall?.copyWith(height: 1.4),
          ),
        ),
      ],
    );
  }

  Widget _logsSection(
    ThemeData theme,
    ColorScheme scheme,
    AppLocalizations l10n, {
    required bool initiallyExpanded,
  }) {
    return Material(
      // ExpansionTile рисует ListTile-заголовок; даём ему собственный Material,
      // чтобы фон корневого Container (DecoratedBox) не глушил ink/splash.
      type: MaterialType.transparency,
      child: Theme(
        // Убираем дефолтные разделители ExpansionTile, чтобы вписать в плотный стек.
        data: theme.copyWith(dividerColor: Colors.transparent),
        child: ExpansionTile(
          initiallyExpanded: initiallyExpanded,
          tilePadding: const EdgeInsets.symmetric(horizontal: 16),
          childrenPadding: const EdgeInsets.fromLTRB(8, 0, 8, 8),
          title: Text(
            l10n.agentMatrixInspectorLogs.toUpperCase(),
            style: theme.textTheme.labelSmall?.copyWith(
              color: scheme.onSurfaceVariant,
              letterSpacing: 0.8,
              fontWeight: FontWeight.w700,
            ),
          ),
          children: [
            SizedBox(
              height: 240,
              child: SandboxLogsViewer(
                projectId: widget.projectId,
                taskId: widget.taskId,
                fillParent: true,
              ),
            ),
          ],
        ),
      ),
    );
  }

  IconData _getRoleIcon(String role) {
    switch (role.toLowerCase()) {
      case 'planner':
        return Icons.architecture;
      case 'decomposer':
        return Icons.account_tree;
      case 'developer':
        return Icons.code;
      case 'reviewer':
        return Icons.rate_review;
      case 'tester':
        return Icons.bug_report;
      case 'merger':
        return Icons.merge_type;
      case 'orchestrator':
        return Icons.hub;
      case 'worker':
        return Icons.build;
      default:
        return Icons.android;
    }
  }
}
