import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_execution_graph.dart';
import 'package:frontend/features/tasks/presentation/widgets/sandbox_logs_viewer.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifact_viewer_dialog.dart';

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
  ConsumerState<AgentInspectorPanel> createState() => _AgentInspectorPanelState();
}

class _AgentInspectorPanelState extends ConsumerState<AgentInspectorPanel>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  int _selectedSubtaskIndex = 0;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
  }

  @override
  void didUpdateWidget(covariant AgentInspectorPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.agent.name != widget.agent.name) {
      _selectedSubtaskIndex = 0;
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agent_inspector_panel');
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    Color statusColor;
    switch (widget.agent.status) {
      case NodeStatus.pending:
        statusColor = Colors.grey;
      case NodeStatus.running:
        statusColor = Colors.blue;
      case NodeStatus.success:
        statusColor = Colors.green;
      case NodeStatus.failed:
        statusColor = Colors.red;
    }

    return Container(
      decoration: BoxDecoration(
        color: theme.scaffoldBackgroundColor,
        border: Border(
          left: BorderSide(color: scheme.outlineVariant, width: 1),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // Header
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
            color: scheme.surfaceContainerLow,
            child: Row(
              children: [
                CircleAvatar(
                  backgroundColor: statusColor.withValues(alpha: 0.1),
                  radius: 18,
                  child: Icon(
                    _getRoleIcon(widget.agent.role),
                    color: statusColor,
                    size: 20,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        widget.agent.name,
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      Text(
                        widget.agent.role.toUpperCase(),
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.outline,
                          fontWeight: FontWeight.w600,
                          fontSize: 10,
                        ),
                      ),
                    ],
                  ),
                ),
                IconButton(
                  onPressed: widget.onClose,
                  icon: const Icon(Icons.close),
                  tooltip: l10n.commonCancel,
                ),
              ],
            ),
          ),
          // Tabs Header
          TabBar(
            controller: _tabController,
            tabs: [
              Tab(text: l10n.agentMatrixInspectorSubtasks),
              Tab(text: l10n.agentMatrixInspectorLogs),
              Tab(text: l10n.agentMatrixInspectorArtifacts),
            ],
            labelStyle: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13),
            unselectedLabelStyle: const TextStyle(fontSize: 13),
          ),
          // Tabs Content
          Expanded(
            child: TabBarView(
              controller: _tabController,
              children: [
                _buildSubtasksTab(context, l10n),
                _buildLogsTab(context, l10n),
                _buildArtifactsTab(context, l10n),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSubtasksTab(BuildContext context, dynamic l10n) {
    final subtasks = widget.agent.subtasks;
    if (subtasks.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Text(
            l10n.agentMatrixInspectorNoSubtasks,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                ),
            textAlign: TextAlign.center,
          ),
        ),
      );
    }

    final theme = Theme.of(context);
    final selectedSubtaskTitle = subtasks[_selectedSubtaskIndex];

    // Ищем описание подзадачи в артефактах
    String description = '';
    for (final art in widget.agent.artifacts) {
      if (art.kind == 'subtask_description' &&
          (art.subtaskTitle == selectedSubtaskTitle || art.summary == selectedSubtaskTitle)) {
        description = art.content?['description'] as String? ?? art.summary;
        break;
      }
    }

    if (description.isEmpty) {
      description = selectedSubtaskTitle;
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Горизонтальный скролл чипсов подзадач
        Container(
          height: 48,
          padding: const EdgeInsets.symmetric(vertical: 8, horizontal: 12),
          child: ListView.builder(
            scrollDirection: Axis.horizontal,
            itemCount: subtasks.length,
            itemBuilder: (context, index) {
              final title = subtasks[index];
              final isSelected = index == _selectedSubtaskIndex;
              return Padding(
                padding: const EdgeInsets.only(right: 8.0),
                child: ChoiceChip(
                  label: Text(
                    title.length > 20 ? '${title.substring(0, 18)}…' : title,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: isSelected ? FontWeight.bold : FontWeight.normal,
                    ),
                  ),
                  selected: isSelected,
                  onSelected: (val) {
                    if (val) {
                      setState(() {
                        _selectedSubtaskIndex = index;
                      });
                    }
                  },
                ),
              );
            },
          ),
        ),
        const Divider(height: 1),
        // Описание подзадачи
        Expanded(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  selectedSubtaskTitle,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  description,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildLogsTab(BuildContext context, dynamic l10n) {
    return Padding(
      padding: const EdgeInsets.all(8.0),
      child: SandboxLogsViewer(
        projectId: widget.projectId,
        taskId: widget.taskId,
        fillParent: true,
      ),
    );
  }

  Widget _buildArtifactsTab(BuildContext context, dynamic l10n) {
    final artifacts = widget.agent.artifacts;
    if (artifacts.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Text(
            l10n.agentMatrixInspectorNoArtifacts,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Colors.grey,
                ),
            textAlign: TextAlign.center,
          ),
        ),
      );
    }

    return ListView.builder(
      padding: const EdgeInsets.all(12),
      itemCount: artifacts.length,
      itemBuilder: (context, index) {
        final art = artifacts[index];
        return Card(
          margin: const EdgeInsets.symmetric(vertical: 4),
          child: ListTile(
            title: Text(
              art.subtaskTitle ?? art.summary,
              style: const TextStyle(fontWeight: FontWeight.bold, fontSize: 13),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            subtitle: Text(
              '${art.kind} · Iteration #${art.iteration}',
              style: const TextStyle(fontSize: 11),
            ),
            trailing: IconButton(
              icon: const Icon(Icons.open_in_new, size: 18),
              onPressed: () {
                showArtifactViewerDialog(
                  context,
                  taskId: widget.taskId,
                  artifactId: art.id,
                );
              },
            ),
          ),
        );
      },
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
      case 'orchestrator':
        return Icons.hub;
      case 'worker':
        return Icons.build;
      default:
        return Icons.android;
    }
  }
}
