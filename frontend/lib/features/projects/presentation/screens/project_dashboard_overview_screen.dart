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
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifacts_dag_section.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_swimlane_trace.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

// ─────────────────────────────────────────────────────────────────────────────
// Группировка статусов задачи → цвет/фильтр (единый язык с трейсом).
// ─────────────────────────────────────────────────────────────────────────────
enum _StatusGroup { active, done, attention, failed, cancelled, unknown }

_StatusGroup _groupOf(String s) => switch (s) {
      'done' || 'completed' => _StatusGroup.done,
      'failed' => _StatusGroup.failed,
      'cancelled' => _StatusGroup.cancelled,
      'needs_human' || 'changes_requested' || 'paused' => _StatusGroup.attention,
      'active' ||
      'in_progress' ||
      'review' ||
      'testing' ||
      'planning' ||
      'pending' =>
        _StatusGroup.active,
      _ => _StatusGroup.unknown,
    };

Color _groupColor(_StatusGroup g) => switch (g) {
      _StatusGroup.active => Colors.blue,
      _StatusGroup.done => const Color(0xFF3FB950),
      _StatusGroup.attention => const Color(0xFFD29922),
      _StatusGroup.failed => Colors.red,
      _StatusGroup.cancelled => Colors.grey,
      _StatusGroup.unknown => Colors.grey,
    };

enum _TaskFilter { all, active, done, issues }

class ProjectDashboardOverviewScreen extends ConsumerStatefulWidget {
  const ProjectDashboardOverviewScreen({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<ProjectDashboardOverviewScreen> createState() =>
      _ProjectDashboardOverviewScreenState();
}

class _ProjectDashboardOverviewScreenState
    extends ConsumerState<ProjectDashboardOverviewScreen> {
  String? _selectedTaskId;
  _TaskFilter _filter = _TaskFilter.all;

  bool _matchesFilter(TaskListItemModel t) {
    final g = _groupOf(t.status);
    return switch (_filter) {
      _TaskFilter.all => true,
      _TaskFilter.active => g == _StatusGroup.active,
      _TaskFilter.done => g == _StatusGroup.done,
      _TaskFilter.issues =>
        g == _StatusGroup.attention || g == _StatusGroup.failed,
    };
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final width = MediaQuery.sizeOf(context).width;
    final isWide = width >= 900;

    final asyncProject = ref.watch(projectProvider(widget.projectId));
    final asyncTasks =
        ref.watch(taskListControllerProvider(projectId: widget.projectId));

    // Инвалидируем артефакты выбранной задачи при смене её статуса (realtime).
    ref.listen<AsyncValue<TaskListState>>(
      taskListControllerProvider(projectId: widget.projectId),
      (previous, next) {
        final prevItems =
            (previous?.hasValue ?? false) ? previous!.requireValue.items : const [];
        final nextItems = next.hasValue ? next.requireValue.items : const [];
        if (_selectedTaskId != null) {
          final prev = prevItems.firstWhereOrNull((t) => t.id == _selectedTaskId);
          final cur = nextItems.firstWhereOrNull((t) => t.id == _selectedTaskId);
          if (prev != null && cur != null && prev.status != cur.status) {
            ref.invalidate(taskArtifactsProvider(_selectedTaskId!));
          }
        }
      },
    );

    return Scaffold(
      body: asyncProject.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => const SizedBox.shrink(),
        data: (project) => asyncTasks.when(
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
          data: (state) => _buildBody(context, l10n, project, state.items, isWide),
        ),
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    AppLocalizations l10n,
    ProjectModel project,
    List<TaskListItemModel> tasks,
    bool isWide,
  ) {
    if (_selectedTaskId == null && tasks.isNotEmpty) {
      _selectedTaskId = tasks.first.id;
    }
    final selected = tasks.firstWhereOrNull((t) => t.id == _selectedTaskId);
    final filtered = tasks.where(_matchesFilter).toList(growable: false);

    final header = _HeaderBar(project: project, tasks: tasks);
    final kpi = _KpiStrip(tasks: tasks, l10n: l10n);

    if (!isWide) {
      return ListView(
        padding: const EdgeInsets.all(16),
        children: [
          header,
          const SizedBox(height: 12),
          kpi,
          const SizedBox(height: 12),
          _FilterChips(
            current: _filter,
            l10n: l10n,
            onChanged: (f) => setState(() => _filter = f),
          ),
          const SizedBox(height: 8),
          if (filtered.isEmpty)
            _EmptyTasks(l10n: l10n)
          else
            for (final t in filtered)
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: _TaskRow(
                  task: t,
                  selected: false,
                  onTap: () => _openTaskSheet(context, l10n, t),
                ),
              ),
        ],
      );
    }

    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
          child: Column(children: [header, const SizedBox(height: 12), kpi]),
        ),
        const SizedBox(height: 12),
        const Divider(height: 1),
        Expanded(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                width: 340,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    Padding(
                      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
                      child: _FilterChips(
                        current: _filter,
                        l10n: l10n,
                        onChanged: (f) => setState(() => _filter = f),
                      ),
                    ),
                    Expanded(
                      child: filtered.isEmpty
                          ? _EmptyTasks(l10n: l10n)
                          : ListView.separated(
                              padding: const EdgeInsets.fromLTRB(12, 0, 12, 16),
                              itemCount: filtered.length,
                              separatorBuilder: (_, _) =>
                                  const SizedBox(height: 4),
                              itemBuilder: (context, i) {
                                final t = filtered[i];
                                return _TaskRow(
                                  task: t,
                                  selected: t.id == _selectedTaskId,
                                  onTap: () =>
                                      setState(() => _selectedTaskId = t.id),
                                );
                              },
                            ),
                    ),
                  ],
                ),
              ),
              const VerticalDivider(width: 1, thickness: 1),
              Expanded(child: _DetailPane(task: selected, l10n: l10n)),
            ],
          ),
        ),
      ],
    );
  }

  void _openTaskSheet(
    BuildContext context,
    AppLocalizations l10n,
    TaskListItemModel task,
  ) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: Theme.of(context).colorScheme.surface,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (context) => DraggableScrollableSheet(
        initialChildSize: 0.7,
        maxChildSize: 0.95,
        minChildSize: 0.4,
        expand: false,
        builder: (context, scrollController) => SingleChildScrollView(
          controller: scrollController,
          padding: const EdgeInsets.all(20),
          child: _DetailPane(task: task, l10n: l10n, scrollable: false),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// HEADER — компактная строка: имя + статус + tech-stack + последняя активность.
// ─────────────────────────────────────────────────────────────────────────────
class _HeaderBar extends StatelessWidget {
  const _HeaderBar({required this.project, required this.tasks});

  final ProjectModel project;
  final List<TaskListItemModel> tasks;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    DateTime? last;
    for (final t in tasks) {
      if (last == null || t.updatedAt.isAfter(last)) {
        last = t.updatedAt;
      }
    }
    last ??= project.updatedAt;
    final localeTag = Localizations.localeOf(context).toLanguageTag();
    final lastStr = _relTime(localeTag, last);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: Row(
        children: [
          Flexible(
            child: Text(
              project.name,
              style: theme.textTheme.titleLarge
                  ?.copyWith(fontWeight: FontWeight.bold),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          const SizedBox(width: 12),
          ProjectStatusChip(status: project.status),
          const Spacer(),
          if (project.techStack.isNotEmpty) ...[
            Flexible(
              child: Wrap(
                alignment: WrapAlignment.end,
                spacing: 6,
                runSpacing: 4,
                children: [
                  for (final e in project.techStack.entries.take(4))
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: scheme.surfaceContainerHighest
                            .withValues(alpha: 0.6),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        '${e.key}: ${e.value}',
                        style: theme.textTheme.labelSmall
                            ?.copyWith(color: scheme.onSurfaceVariant),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 12),
          ],
          Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.schedule, size: 14, color: scheme.onSurfaceVariant),
              const SizedBox(width: 4),
              Text(lastStr,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(color: scheme.onSurfaceVariant)),
            ],
          ),
        ],
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// KPI — счётчики по статусам + прогресс-бар проекта.
// ─────────────────────────────────────────────────────────────────────────────
class _KpiStrip extends StatelessWidget {
  const _KpiStrip({required this.tasks, required this.l10n});

  final List<TaskListItemModel> tasks;
  final AppLocalizations l10n;

  int _count(_StatusGroup g) =>
      tasks.where((t) => _groupOf(t.status) == g).length;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final total = tasks.length;
    final done = _count(_StatusGroup.done);
    final progress = total == 0 ? 0.0 : done / total;

    Widget card(String label, int n, Color accent) => Expanded(
          child: Container(
            margin: const EdgeInsets.only(right: 8),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            decoration: BoxDecoration(
              color: scheme.surfaceContainerLow,
              borderRadius: BorderRadius.circular(12),
              border: Border.all(color: scheme.outlineVariant),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(children: [
                  Container(
                    width: 8,
                    height: 8,
                    decoration:
                        BoxDecoration(color: accent, shape: BoxShape.circle),
                  ),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(label,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.labelSmall
                            ?.copyWith(color: scheme.onSurfaceVariant)),
                  ),
                ]),
                const SizedBox(height: 6),
                Text('$n',
                    style: theme.textTheme.headlineSmall
                        ?.copyWith(fontWeight: FontWeight.bold)),
              ],
            ),
          ),
        );

    return Column(
      children: [
        Row(
          children: [
            card(l10n.projectKpiTotal, total, scheme.primary),
            card(l10n.projectKpiActive, _count(_StatusGroup.active),
                _groupColor(_StatusGroup.active)),
            card(l10n.projectKpiDone, done, _groupColor(_StatusGroup.done)),
            card(l10n.projectKpiAttention, _count(_StatusGroup.attention),
                _groupColor(_StatusGroup.attention)),
            card(l10n.projectKpiFailed, _count(_StatusGroup.failed),
                _groupColor(_StatusGroup.failed)),
          ],
        ),
        const SizedBox(height: 10),
        Row(
          children: [
            Text(
              '${l10n.projectKpiDone}  $done/$total',
              style: theme.textTheme.bodySmall
                  ?.copyWith(color: scheme.onSurfaceVariant),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(4),
                child: LinearProgressIndicator(
                  value: progress,
                  minHeight: 6,
                  backgroundColor: scheme.surfaceContainerHighest,
                  valueColor: const AlwaysStoppedAnimation(Color(0xFF3FB950)),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Text('${(progress * 100).round()}%',
                style: theme.textTheme.bodySmall?.copyWith(
                    color: scheme.onSurfaceVariant,
                    fontFeatures: const [FontFeature.tabularFigures()])),
          ],
        ),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// FILTER CHIPS
// ─────────────────────────────────────────────────────────────────────────────
class _FilterChips extends StatelessWidget {
  const _FilterChips({
    required this.current,
    required this.onChanged,
    required this.l10n,
  });

  final _TaskFilter current;
  final ValueChanged<_TaskFilter> onChanged;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    Widget chip(_TaskFilter f, String label) => Padding(
          padding: const EdgeInsets.only(right: 6),
          child: ChoiceChip(
            visualDensity: VisualDensity.compact,
            label: Text(label, style: const TextStyle(fontSize: 12)),
            selected: current == f,
            onSelected: (_) => onChanged(f),
          ),
        );
    return Wrap(
      children: [
        chip(_TaskFilter.all, l10n.projectTaskFilterAll),
        chip(_TaskFilter.active, l10n.projectKpiActive),
        chip(_TaskFilter.done, l10n.projectKpiDone),
        chip(_TaskFilter.issues, l10n.projectTaskFilterIssues),
      ],
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// TASK ROW — плотная строка.
// ─────────────────────────────────────────────────────────────────────────────
class _TaskRow extends StatelessWidget {
  const _TaskRow({
    required this.task,
    required this.selected,
    required this.onTap,
  });

  final TaskListItemModel task;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final group = _groupOf(task.status);
    final accent = _groupColor(group);
    final localeTag = Localizations.localeOf(context).toLanguageTag();
    final prTone = taskPriorityTone(task.priority);

    return Material(
      color: selected ? scheme.surfaceContainerHigh : Colors.transparent,
      borderRadius: BorderRadius.circular(10),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(10),
            border: Border(
              left: BorderSide(
                color: selected ? scheme.primary : Colors.transparent,
                width: 2,
              ),
            ),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Container(
                  width: 9,
                  height: 9,
                  decoration:
                      BoxDecoration(color: accent, shape: BoxShape.circle),
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      task.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontWeight:
                            selected ? FontWeight.w600 : FontWeight.w500,
                      ),
                    ),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        Icon(taskPriorityIcon(prTone),
                            size: 13, color: scheme.onSurfaceVariant),
                        const SizedBox(width: 2),
                        Text(
                          taskStatusLabel(
                              AppLocalizations.of(context)!, task.status),
                          style: theme.textTheme.labelSmall
                              ?.copyWith(color: accent),
                        ),
                        const Spacer(),
                        Text(
                          _relTime(localeTag, task.updatedAt),
                          style: theme.textTheme.labelSmall
                              ?.copyWith(color: scheme.onSurfaceVariant),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// DETAIL PANE — заголовок + мини-трейс + артефакты.
// ─────────────────────────────────────────────────────────────────────────────
class _DetailPane extends ConsumerWidget {
  const _DetailPane({
    required this.task,
    required this.l10n,
    this.scrollable = true,
  });

  final TaskListItemModel? task;
  final AppLocalizations l10n;
  final bool scrollable;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final t = task;
    if (t == null) {
      return Center(
        child: Text(l10n.tasksEmpty,
            style: theme.textTheme.bodyLarge
                ?.copyWith(color: scheme.outline)),
      );
    }

    final accent = _groupColor(_groupOf(t.status));
    final prTone = taskPriorityTone(t.priority);

    final children = <Widget>[
      Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Text(t.title,
                style: theme.textTheme.titleLarge
                    ?.copyWith(fontWeight: FontWeight.bold)),
          ),
          const SizedBox(width: 12),
          FilledButton.tonalIcon(
            onPressed: () =>
                context.push('/projects/${t.projectId}/tasks/${t.id}'),
            icon: const Icon(Icons.open_in_new, size: 16),
            label: Text(l10n.projectOpenTask),
          ),
        ],
      ),
      const SizedBox(height: 12),
      Wrap(
        spacing: 8,
        runSpacing: 6,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          _pill(accent.withValues(alpha: 0.16), accent,
              taskStatusLabel(l10n, t.status)),
          _pill(
            scheme.surfaceContainerHighest.withValues(alpha: 0.6),
            scheme.onSurfaceVariant,
            taskPriorityLabel(l10n, t.priority),
            icon: taskPriorityIcon(prTone),
          ),
          if (t.branchName != null && t.branchName!.isNotEmpty)
            _pill(
              scheme.surfaceContainerHighest.withValues(alpha: 0.6),
              scheme.onSurfaceVariant,
              t.branchName!,
              icon: Icons.commit,
            ),
        ],
      ),
      const SizedBox(height: 16),
      Text(l10n.taskVizTabTrace,
          style: theme.textTheme.titleSmall
              ?.copyWith(fontWeight: FontWeight.bold)),
      const SizedBox(height: 8),
      Container(
        height: 240,
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: scheme.outlineVariant),
        ),
        clipBehavior: Clip.antiAlias,
        child: TaskSwimlaneTrace(
          key: ValueKey('mini-trace-${t.id}'),
          projectId: t.projectId,
          taskId: t.id,
          taskState: t.status,
          showLegend: false,
          onAgentSelected: (_) {},
        ),
      ),
      const SizedBox(height: 20),
      Text(l10n.artifactsSection,
          style: theme.textTheme.titleSmall
              ?.copyWith(fontWeight: FontWeight.bold)),
      const SizedBox(height: 8),
      ArtifactsDagSection(taskId: t.id),
    ];

    if (!scrollable) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: children,
      );
    }
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: children,
      ),
    );
  }

  Widget _pill(Color bg, Color fg, String text, {IconData? icon}) => Builder(
        builder: (context) => Container(
          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
          decoration:
              BoxDecoration(color: bg, borderRadius: BorderRadius.circular(6)),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (icon != null) ...[
                Icon(icon, size: 13, color: fg),
                const SizedBox(width: 4),
              ],
              Text(text,
                  style: Theme.of(context)
                      .textTheme
                      .labelMedium
                      ?.copyWith(color: fg, fontWeight: FontWeight.w600)),
            ],
          ),
        ),
      );
}

class _EmptyTasks extends StatelessWidget {
  const _EmptyTasks({required this.l10n});
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.assignment_outlined,
                size: 48, color: theme.colorScheme.outline),
            const SizedBox(height: 16),
            Text(l10n.tasksEmpty,
                style: theme.textTheme.bodyLarge
                    ?.copyWith(color: theme.colorScheme.onSurfaceVariant)),
          ],
        ),
      ),
    );
  }
}

String _relTime(String localeTag, DateTime dt) {
  final local = dt.toLocal();
  final now = DateTime.now();
  final sameDay =
      local.year == now.year && local.month == now.month && local.day == now.day;
  return sameDay
      ? DateFormat.Hm(localeTag).format(local)
      : DateFormat.MMMd(localeTag).format(local);
}
