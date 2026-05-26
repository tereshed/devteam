import 'package:collection/collection.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/task_list_item_model.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifacts_dag_section.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_card.dart';
import 'package:frontend/l10n/app_localizations.dart';

class ProjectDashboardOverviewScreen extends ConsumerStatefulWidget {
  const ProjectDashboardOverviewScreen({
    super.key,
    required this.projectId,
  });

  final String projectId;

  @override
  ConsumerState<ProjectDashboardOverviewScreen> createState() =>
      _ProjectDashboardOverviewScreenState();
}

class _ProjectDashboardOverviewScreenState
    extends ConsumerState<ProjectDashboardOverviewScreen> {
  String? _selectedTaskId;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final width = MediaQuery.sizeOf(context).width;
    final isWide = width >= 900;

    final asyncProject = ref.watch(projectProvider(widget.projectId));
    final asyncTasks = ref.watch(taskListControllerProvider(projectId: widget.projectId));

    // Listen for task changes to invalidating artifacts of the selected task on state updates.
    ref.listen<AsyncValue<TaskListState>>(
      taskListControllerProvider(projectId: widget.projectId),
      (previous, next) {
        final prevVal = (previous != null && previous.hasValue) ? previous.requireValue : null;
        final nextVal = next.hasValue ? next.requireValue : null;
        final prevItems = prevVal?.items ?? [];
        final nextItems = nextVal?.items ?? [];
        if (_selectedTaskId != null) {
          final prevTask = prevItems.firstWhereOrNull((t) => t.id == _selectedTaskId);
          final nextTask = nextItems.firstWhereOrNull((t) => t.id == _selectedTaskId);
          if (prevTask != null && nextTask != null && prevTask.status != nextTask.status) {
            ref.invalidate(taskArtifactsProvider(_selectedTaskId!));
          }
        }
      },
    );

    return Scaffold(
      body: asyncProject.when(
        loading: () => const SizedBox.shrink(),
        error: (err, _) => const SizedBox.shrink(),
        data: (project) {
          return asyncTasks.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => Center(
              child: DataLoadErrorMessage(
                title: l10n.dataLoadError,
                actionLabel: l10n.retry,
                onAction: () => ref.invalidate(
                  taskListControllerProvider(projectId: widget.projectId),
                ),
              ),
            ),
            data: (tasksState) {
              final tasks = tasksState.items;

              // Auto-select the first task if none is currently selected.
              if (_selectedTaskId == null && tasks.isNotEmpty) {
                _selectedTaskId = tasks.first.id;
              }

              final selectedTask = tasks.firstWhereOrNull((t) => t.id == _selectedTaskId);

              if (isWide) {
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    // Left Column: Project Overview and Tasks List
                    Expanded(
                      flex: 2,
                      child: SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.stretch,
                          children: [
                            _buildProjectInfoCard(context, project, l10n),
                            const SizedBox(height: 16),
                            _buildTasksHeader(context, tasks.length, l10n),
                            const SizedBox(height: 8),
                            if (tasks.isEmpty)
                              _buildEmptyTasksState(context, l10n)
                            else
                              ListView.separated(
                                shrinkWrap: true,
                                physics: const NeverScrollableScrollPhysics(),
                                itemCount: tasks.length,
                                separatorBuilder: (_, __) => const SizedBox(height: 8),
                                itemBuilder: (context, index) {
                                  final task = tasks[index];
                                  final isSelected = task.id == _selectedTaskId;
                                  return Container(
                                    decoration: BoxDecoration(
                                      borderRadius: BorderRadius.circular(12),
                                      border: isSelected
                                          ? Border.all(
                                              color: theme.colorScheme.primary,
                                              width: 2,
                                            )
                                          : null,
                                    ),
                                    child: TaskCard(
                                      task: task,
                                      onTap: () {
                                        setState(() {
                                          _selectedTaskId = task.id;
                                        });
                                      },
                                    ),
                                  );
                                },
                              ),
                          ],
                        ),
                      ),
                    ),
                    const VerticalDivider(width: 1, thickness: 1),
                    // Right Column: Selected Task Details and Artifacts
                    Expanded(
                      flex: 3,
                      child: _buildTaskDetailPane(context, selectedTask, l10n),
                    ),
                  ],
                );
              } else {
                // Mobile/Narrow screen layout: single-scroll list, opens bottom sheet for details.
                return SingleChildScrollView(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _buildProjectInfoCard(context, project, l10n),
                      const SizedBox(height: 16),
                      _buildTasksHeader(context, tasks.length, l10n),
                      const SizedBox(height: 8),
                      if (tasks.isEmpty)
                        _buildEmptyTasksState(context, l10n)
                      else
                        ListView.separated(
                          shrinkWrap: true,
                          physics: const NeverScrollableScrollPhysics(),
                          itemCount: tasks.length,
                          separatorBuilder: (_, __) => const SizedBox(height: 8),
                          itemBuilder: (context, index) {
                            final task = tasks[index];
                            return TaskCard(
                              task: task,
                              onTap: () => _showTaskDetailsBottomSheet(context, task, l10n),
                            );
                          },
                        ),
                    ],
                  ),
                );
              }
            },
          );
        },
      ),
    );
  }

  Widget _buildProjectInfoCard(
    BuildContext context,
    ProjectModel project,
    AppLocalizations l10n,
  ) {
    final theme = Theme.of(context);
    return Card(
      elevation: 0,
      color: theme.colorScheme.surfaceContainerLow,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(16),
        side: BorderSide(color: theme.colorScheme.outlineVariant),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Text(
                    project.name,
                    style: theme.textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                ProjectStatusChip(status: project.status),
              ],
            ),
            if (project.description.isNotEmpty) ...[
              const SizedBox(height: 12),
              Text(
                project.description,
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
            ],
            if (project.techStack.isNotEmpty) ...[
              const SizedBox(height: 16),
              Text(
                l10n.projectSettingsSectionTechStack,
                style: theme.textTheme.titleSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: project.techStack.entries.map((e) {
                  return Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.secondaryContainer,
                      borderRadius: BorderRadius.circular(8),
                      border: Border.all(color: theme.colorScheme.outlineVariant),
                    ),
                    child: Text(
                      '${e.key}: ${e.value}',
                      style: theme.textTheme.labelMedium?.copyWith(
                        color: theme.colorScheme.onSecondaryContainer,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  );
                }).toList(),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTasksHeader(
    BuildContext context,
    int count,
    AppLocalizations l10n,
  ) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 4),
      child: Row(
        children: [
          Text(
            l10n.projectDashboardTasks,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(width: 8),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
            decoration: BoxDecoration(
              color: theme.colorScheme.primary.withValues(alpha: 0.1),
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              '$count',
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.primary,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyTasksState(BuildContext context, AppLocalizations l10n) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32.0),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.assignment_outlined,
              size: 48,
              color: theme.colorScheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              l10n.tasksEmpty,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTaskDetailPane(
    BuildContext context,
    TaskListItemModel? task,
    AppLocalizations l10n,
  ) {
    final theme = Theme.of(context);
    if (task == null) {
      return Center(
        child: Text(
          l10n.tasksEmpty,
          style: theme.textTheme.bodyLarge?.copyWith(
            color: theme.colorScheme.outline,
          ),
        ),
      );
    }

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            task.title,
            style: theme.textTheme.headlineSmall?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 16),
          // We can't display a description if the task list model doesn't have it, but wait,
          // list item model might not contain descriptions. Let's inspect what is on task model.
          // Wait, is description in TaskListItemModel? If not, we can show status & artifacts.
          // Let's use the DAG Section for artifacts.
          Text(
            l10n.artifactsSection,
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 8),
          ArtifactsDagSection(taskId: task.id),
        ],
      ),
    );
  }

  void _showTaskDetailsBottomSheet(
    BuildContext context,
    TaskListItemModel task,
    AppLocalizations l10n,
  ) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (context) {
        return DraggableScrollableSheet(
          initialChildSize: 0.6,
          maxChildSize: 0.9,
          minChildSize: 0.4,
          expand: false,
          builder: (context, scrollController) {
            return SingleChildScrollView(
              controller: scrollController,
              padding: const EdgeInsets.all(24),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Center(
                    child: Container(
                      width: 40,
                      height: 4,
                      decoration: BoxDecoration(
                        color: Theme.of(context).colorScheme.outlineVariant,
                        borderRadius: BorderRadius.circular(2),
                      ),
                    ),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    task.title,
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                  ),
                  const SizedBox(height: 24),
                  Text(
                    l10n.artifactsSection,
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                  ),
                  const SizedBox(height: 8),
                  ArtifactsDagSection(taskId: task.id),
                ],
              ),
            );
          },
        );
      },
    );
  }
}
