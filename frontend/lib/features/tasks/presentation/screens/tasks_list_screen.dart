import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/domain/models/task_list_item_model.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_card.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

TaskListState? _unwrapTaskListState(AsyncValue<TaskListState>? async) {
  if (async == null || !async.hasValue) {
    return null;
  }
  return async.requireValue;
}

/// Дебаунс поля поиска на списке задач (как [kProjectsListSearchDebounce], 10.4).
const Duration kTasksListSearchDebounce = Duration(milliseconds: 400);

/// Порог ширины: список / Kanban (10.4, frontend § 2.2).
const double kTasksListMobileBreakpointWidth = 600;

/// Порог «докрутки» для [TaskListController.loadMore] (п. 7 задачи 12.4).
const double kTasksListLoadMoreExtentAfterPx = 600;

/// Виджет-тесты: инлайн-баннер `loadMoreError` (не путать со [SnackBar] с тем же `retry`).
const ValueKey<String> kTasksListLoadMoreErrorBannerKey =
    ValueKey<String>('tasks_list_load_more_error_banner');

/// Группировка строк списка по колонкам Kanban за один проход (см. FAQ п. 9 в 12.4).
Map<String, List<TaskListItemModel>> _groupTaskItemsForKanban(
  List<TaskListItemModel> items,
) {
  final grouped = <String, List<TaskListItemModel>>{
    for (final s in taskStatuses) s: <TaskListItemModel>[],
  };
  for (final t in items) {
    grouped[t.status]?.add(t);
  }
  return grouped;
}

/// Список задач проекта: фильтры, список или Kanban — без собственного [Scaffold]/[AppBar].
class TasksListScreen extends ConsumerStatefulWidget {
  const TasksListScreen({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<TasksListScreen> createState() => _TasksListScreenState();
}

class _TasksListScreenState extends ConsumerState<TasksListScreen> {
  /// Как [ProjectsListScreen._kRefreshTimeout] (10.4); вынести в общую константу — отдельная задача.
  static const Duration _kRefreshTimeout = Duration(seconds: 30);

  /// Кэш эталона [TaskListFilter.defaults] для сравнения без аллокаций на каждый кадр.
  static final TaskListFilter _kDefaultListFilter = TaskListFilter.defaults();

  final _searchController = TextEditingController();
  Timer? _searchDebounce;
  TaskListState? _lastSeenState;

  @override
  void dispose() {
    _searchDebounce?.cancel();
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged(String value) {
    _searchDebounce?.cancel();
    _searchDebounce = Timer(kTasksListSearchDebounce, () {
      if (!mounted) {
        return;
      }
      final trimmed = value.trim();
      final async = ref.read(taskListControllerProvider(projectId: widget.projectId));
      final cur = _unwrapTaskListState(async)?.filter;
      if (cur == null) {
        return;
      }
      final nextSearch = trimmed.isEmpty ? null : trimmed;
      if (cur.search == nextSearch) {
        return;
      }
      unawaited(
        ref
            .read(taskListControllerProvider(projectId: widget.projectId).notifier)
            .setFilter(cur.copyWith(search: nextSearch)),
      );
    });
  }

  Future<void> _onRefresh() async {
    try {
      await ref
          .read(taskListControllerProvider(projectId: widget.projectId).notifier)
          .refresh()
          .timeout(_kRefreshTimeout);
    } on TimeoutException {
      // SnackBar при частичной ошибке — через ref.listen (если провайдер сохранит value).
    } on Exception {
      // Сеть / домен; не глотаем Error.
    }
  }

  bool _filterDiffersFromDefaults(TaskListFilter f) => f != _kDefaultListFilter;

  void _maybeTriggerLoadMore(TaskListState state) {
    if (!state.hasMore || state.isLoadingMore || state.isLoadingInitial) {
      return;
    }
    unawaited(
      ref.read(taskListControllerProvider(projectId: widget.projectId).notifier).loadMore(),
    );
  }

  bool _onScrollLoadMore(TaskListState state, ScrollNotification n) {
    final m = n.metrics;
    if (m.maxScrollExtent <= 0) {
      return false;
    }
    if (m.extentAfter >= kTasksListLoadMoreExtentAfterPx) {
      return false;
    }
    _maybeTriggerLoadMore(state);
    return false;
  }

  Future<void> _applyStatusFilter(String? statusOrNull) async {
    final async = ref.read(taskListControllerProvider(projectId: widget.projectId));
    final cur = _unwrapTaskListState(async)?.filter;
    if (cur == null) {
      return;
    }
    if (cur.status == statusOrNull) {
      return;
    }
    await ref
        .read(taskListControllerProvider(projectId: widget.projectId).notifier)
        .setFilter(cur.copyWith(status: statusOrNull));
  }

  Future<void> _applyPriorityFilter(String? priorityOrNull) async {
    final async = ref.read(taskListControllerProvider(projectId: widget.projectId));
    final cur = _unwrapTaskListState(async)?.filter;
    if (cur == null) {
      return;
    }
    if (cur.priority == priorityOrNull) {
      return;
    }
    await ref
        .read(taskListControllerProvider(projectId: widget.projectId).notifier)
        .setFilter(cur.copyWith(priority: priorityOrNull));
  }

  Future<void> _clearFiltersToDefaults() async {
    await ref
        .read(taskListControllerProvider(projectId: widget.projectId).notifier)
        .setFilter(TaskListFilter.defaults());
    _searchController.clear();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final width = MediaQuery.sizeOf(context).width;
    final isWide = width >= kTasksListMobileBreakpointWidth;

    final async = ref.watch(taskListControllerProvider(projectId: widget.projectId));

    ref.listen(taskListControllerProvider(projectId: widget.projectId), (prev, next) {
      final prevSearch = _unwrapTaskListState(prev)?.filter.search;
      final nextSearch = _unwrapTaskListState(next)?.filter.search;
      if (prevSearch != nextSearch) {
        final target = nextSearch ?? '';
        if (_searchController.text != target) {
          // Программная запись в controller не вызывает TextField.onChanged —
          // лишний дебаунс от WS/refetch не запускается (см. идемпотентность в _onSearchChanged).
          _searchController.value = TextEditingValue(
            text: target,
            selection: TextSelection.collapsed(offset: target.length),
          );
        }
      }

      if (next.hasValue) {
        _lastSeenState = next.value;
      }

      // Ошибка refresh при уже показанных данных (10.4). Retry = полный refresh().
      // Если когда-то одновременно появятся AsyncError и loadMoreError — уточнить UX (12.5).
      if (next.hasError && next.hasValue && context.mounted) {
        final err = next.error!;
        final detail = taskErrorDetail(err);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(taskListErrorTitle(l10n, err)),
                if (detail != null) Text(detail),
              ],
            ),
            action: SnackBarAction(
              label: l10n.retry,
              onPressed: () => unawaited(
                ref
                    .read(taskListControllerProvider(projectId: widget.projectId).notifier)
                    .refresh(),
              ),
            ),
          ),
        );
      }
    });

    final effective = _unwrapTaskListState(async) ?? _lastSeenState;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _TasksFilterPanel(
          l10n: l10n,
          searchController: _searchController,
          onSearchChanged: _onSearchChanged,
          filter: effective?.filter,
          onStatusSelected: _applyStatusFilter,
          onPrioritySelected: _applyPriorityFilter,
          showTrailingRefresh: isWide,
          onTrailingRefresh: _onRefresh,
        ),
        Expanded(
          child: _TasksListBody(
            async: async,
            effective: effective,
            l10n: l10n,
            isWide: isWide,
            filterDiffersFromDefaults: effective != null &&
                _filterDiffersFromDefaults(effective.filter),
            onRefresh: _onRefresh,
            onRetryFullError: _onRefresh,
            onClearFilters: _clearFiltersToDefaults,
            onScrollLoadMore: _onScrollLoadMore,
            onLoadMoreRetry: () => unawaited(
              ref
                  .read(taskListControllerProvider(projectId: widget.projectId).notifier)
                  .loadMore(),
            ),
            onTaskTap: (task) {
              context.push('/projects/${widget.projectId}/tasks/${task.id}');
            },
          ),
        ),
      ],
    );
  }
}

class _TasksFilterPanel extends StatelessWidget {
  const _TasksFilterPanel({
    required this.l10n,
    required this.searchController,
    required this.onSearchChanged,
    required this.filter,
    required this.onStatusSelected,
    required this.onPrioritySelected,
    required this.showTrailingRefresh,
    required this.onTrailingRefresh,
  });

  final AppLocalizations l10n;
  final TextEditingController searchController;
  final ValueChanged<String> onSearchChanged;
  final TaskListFilter? filter;
  final Future<void> Function(String?) onStatusSelected;
  final Future<void> Function(String?) onPrioritySelected;
  final bool showTrailingRefresh;
  final Future<void> Function() onTrailingRefresh;

  @override
  Widget build(BuildContext context) {
    final st = filter?.status;
    final pr = filter?.priority;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: TextField(
                  controller: searchController,
                  onChanged: onSearchChanged,
                  decoration: InputDecoration(
                    hintText: l10n.tasksSearchHint,
                    prefixIcon: const Icon(Icons.search),
                    border: const OutlineInputBorder(),
                    isDense: true,
                  ),
                ),
              ),
              if (showTrailingRefresh) ...[
                const SizedBox(width: 8),
                IconButton(
                  tooltip: MaterialLocalizations.of(context).refreshIndicatorSemanticLabel,
                  onPressed: () => onTrailingRefresh(),
                  icon: const Icon(Icons.refresh),
                ),
              ],
            ],
          ),
          const SizedBox(height: 8),
          SizedBox(
            height: 44,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: taskStatuses.length + 1,
              separatorBuilder: (_, _) => const SizedBox(width: 8),
              itemBuilder: (context, i) {
                final sel = i == 0 ? null : taskStatuses[i - 1];
                final selected = st == sel;
                final label =
                    i == 0 ? l10n.filterAll : taskStatusLabel(l10n, sel!);
                return FilterChip(
                  label: Text(label),
                  selected: selected,
                  onSelected: (_) => onStatusSelected(sel),
                );
              },
            ),
          ),
          const SizedBox(height: 8),
          SizedBox(
            height: 44,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: taskPriorities.length + 1,
              separatorBuilder: (_, _) => const SizedBox(width: 8),
              itemBuilder: (context, i) {
                final sel = i == 0 ? null : taskPriorities[i - 1];
                final selected = pr == sel;
                final label =
                    i == 0 ? l10n.filterAll : taskPriorityLabel(l10n, sel!);
                return FilterChip(
                  label: Text(label),
                  selected: selected,
                  onSelected: (_) => onPrioritySelected(sel),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _TasksListBody extends StatelessWidget {
  const _TasksListBody({
    required this.async,
    required this.effective,
    required this.l10n,
    required this.isWide,
    required this.filterDiffersFromDefaults,
    required this.onRefresh,
    required this.onRetryFullError,
    required this.onClearFilters,
    required this.onScrollLoadMore,
    required this.onLoadMoreRetry,
    required this.onTaskTap,
  });

  final AsyncValue<TaskListState> async;
  final TaskListState? effective;
  final AppLocalizations l10n;
  final bool isWide;
  final bool filterDiffersFromDefaults;
  final Future<void> Function() onRefresh;
  final Future<void> Function() onRetryFullError;
  final Future<void> Function() onClearFilters;
  final bool Function(TaskListState state, ScrollNotification n) onScrollLoadMore;
  final VoidCallback onLoadMoreRetry;
  final void Function(TaskListItemModel task) onTaskTap;

  @override
  Widget build(BuildContext context) {
    if (async.hasError && effective == null) {
      final err = async.error!;
      final detail = taskErrorDetail(err);
      return _TasksFullBleedMessage(
        icon: Icons.error_outline,
        iconColor: Theme.of(context).colorScheme.error,
        title: taskListErrorTitle(l10n, err),
        subtitle: detail,
        actionLabel: l10n.retry,
        onAction: onRetryFullError,
      );
    }

    final state = effective;
    if (state == null) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.isLoadingInitial && state.items.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (!state.isLoadingInitial && state.items.isEmpty) {
      final filtered = filterDiffersFromDefaults;
      final emptyBody = CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverFillRemaining(
            hasScrollBody: false,
            child: _TasksEmptyPane(
              filtered: filtered,
              l10n: l10n,
              onClearFilters: filtered ? onClearFilters : null,
            ),
          ),
        ],
      );
      // П. 6 UI 12.4: на wide без RefreshIndicator — только IconButton в фильтр-панели.
      return isWide
          ? emptyBody
          : RefreshIndicator(
              onRefresh: onRefresh,
              child: emptyBody,
            );
    }

    if (isWide) {
      // Kanban: вертикальный ScrollNotification приходит только из «достаточно длинной»
      // колонки; в коротких колонках ListView не скроллится — см. FAQ 12.4 §частые ошибки №11.
      // Дополнительно слушаем горизонтальный SingleChildScrollView доски (extentAfter вправо).
      // Если доска и колонки полностью помещаются без скролла, loadMore по уведомлениям не
      // сработает (редкий desktop-кейс); отложено: auto-prefetch при hasMore (задача 12.3).
      final grouped = _groupTaskItemsForKanban(state.items);
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (state.loadMoreError != null)
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              child: _LoadMoreErrorBanner(
                l10n: l10n,
                error: state.loadMoreError!,
                onRetry: onLoadMoreRetry,
              ),
            ),
          Expanded(
            child: LayoutBuilder(
              builder: (context, constraints) {
                return NotificationListener<ScrollNotification>(
                  onNotification: (ScrollNotification n) {
                    if (n.metrics.axis != Axis.horizontal) {
                      return false;
                    }
                    return onScrollLoadMore(state, n);
                  },
                  child: SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    child: SizedBox(
                      height: constraints.maxHeight,
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          for (final status in taskStatuses)
                            _KanbanColumn(
                              statusWire: status,
                              height: constraints.maxHeight,
                              tasks: grouped[status]!,
                              l10n: l10n,
                              onTaskTap: onTaskTap,
                              onScrollNotification: (n) =>
                                  onScrollLoadMore(state, n),
                            ),
                        ],
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
          if (state.isLoadingMore)
            const Padding(
              padding: EdgeInsets.all(8),
              child: Center(child: CircularProgressIndicator()),
            ),
        ],
      );
    }

    final scrollable = NotificationListener<ScrollNotification>(
      onNotification: (n) => onScrollLoadMore(state, n),
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (state.loadMoreError != null)
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
                child: _LoadMoreErrorBanner(
                  l10n: l10n,
                  error: state.loadMoreError!,
                  onRetry: onLoadMoreRetry,
                ),
              ),
            ),
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            sliver: SliverList(
              delegate: SliverChildBuilderDelegate(
                (context, index) {
                  if (index >= state.items.length) {
                    return const Padding(
                      padding: EdgeInsets.symmetric(vertical: 16),
                      child: Center(child: CircularProgressIndicator()),
                    );
                  }
                  final task = state.items[index];
                  return Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: TaskCard(task: task, onTap: () => onTaskTap(task)),
                  );
                },
                childCount: state.items.length + (state.isLoadingMore ? 1 : 0),
              ),
            ),
          ),
        ],
      ),
    );

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: scrollable,
    );
  }
}

class _KanbanColumn extends StatelessWidget {
  const _KanbanColumn({
    required this.statusWire,
    required this.height,
    required this.tasks,
    required this.l10n,
    required this.onTaskTap,
    required this.onScrollNotification,
  });

  final String statusWire;
  final double height;
  final List<TaskListItemModel> tasks;
  final AppLocalizations l10n;
  final void Function(TaskListItemModel task) onTaskTap;
  final bool Function(ScrollNotification n) onScrollNotification;

  static const double _kColumnWidth = 280;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final tone = taskStatusTone(statusWire);
    final headerBg = taskStatusChipBackground(scheme, tone);
    final headerFg = taskStatusChipForeground(scheme, tone);

    return SizedBox(
      width: _kColumnWidth,
      height: height,
      child: Padding(
        padding: const EdgeInsets.only(right: 12),
        child: DecoratedBox(
          decoration: BoxDecoration(
            border: Border.all(color: scheme.outlineVariant),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                decoration: BoxDecoration(
                  color: headerBg,
                  borderRadius: const BorderRadius.vertical(top: Radius.circular(11)),
                ),
                child: Row(
                  children: [
                    Icon(taskStatusIcon(tone), size: 18, color: headerFg),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        taskStatusLabel(l10n, statusWire),
                        style: Theme.of(context).textTheme.titleSmall?.copyWith(color: headerFg),
                      ),
                    ),
                    Text(
                      '${tasks.length}',
                      style: Theme.of(context).textTheme.labelMedium?.copyWith(color: headerFg),
                    ),
                  ],
                ),
              ),
              Expanded(
                child: NotificationListener<ScrollNotification>(
                  onNotification: onScrollNotification,
                  child: ListView.builder(
                    padding: const EdgeInsets.all(8),
                    itemCount: tasks.length,
                    itemBuilder: (context, i) {
                      final task = tasks[i];
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 8),
                        child: TaskCard(
                          task: task,
                          dense: true,
                          onTap: () => onTaskTap(task),
                        ),
                      );
                    },
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _LoadMoreErrorBanner extends StatelessWidget {
  const _LoadMoreErrorBanner({
    required this.l10n,
    required this.error,
    required this.onRetry,
  });

  final AppLocalizations l10n;
  final Object error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final detail = taskErrorDetail(error);
    return Material(
      key: kTasksListLoadMoreErrorBannerKey,
      color: scheme.errorContainer,
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              taskListErrorTitle(l10n, error),
              style: Theme.of(context).textTheme.titleSmall?.copyWith(
                    color: scheme.onErrorContainer,
                  ),
            ),
            if (detail != null) ...[
              const SizedBox(height: 4),
              Text(
                detail,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                      color: scheme.onErrorContainer,
                    ),
              ),
            ],
            Align(
              alignment: Alignment.centerRight,
              child: TextButton.icon(
                onPressed: onRetry,
                icon: const Icon(Icons.refresh),
                label: Text(l10n.retry),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _TasksEmptyPane extends StatelessWidget {
  const _TasksEmptyPane({
    required this.filtered,
    required this.l10n,
    required this.onClearFilters,
  });

  final bool filtered;
  final AppLocalizations l10n;
  final Future<void> Function()? onClearFilters;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            filtered ? Icons.search_off : Icons.inbox_outlined,
            size: 56,
            color: Theme.of(context).colorScheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            filtered ? l10n.tasksEmptyFiltered : l10n.tasksEmpty,
            style: Theme.of(context).textTheme.bodyLarge,
            textAlign: TextAlign.center,
          ),
          if (filtered && onClearFilters != null) ...[
            const SizedBox(height: 16),
            FilledButton(
              onPressed: () => onClearFilters!(),
              child: Text(l10n.tasksEmptyFilteredClear),
            ),
          ],
        ],
      ),
    );
  }
}

class _TasksFullBleedMessage extends StatelessWidget {
  const _TasksFullBleedMessage({
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.subtitle,
    required this.actionLabel,
    required this.onAction,
  });

  final IconData icon;
  final Color iconColor;
  final String title;
  final String? subtitle;
  final String actionLabel;
  final Future<void> Function() onAction;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 48, color: iconColor),
            const SizedBox(height: 12),
            Text(title, textAlign: TextAlign.center),
            if (subtitle != null) ...[
              const SizedBox(height: 8),
              Text(subtitle!, textAlign: TextAlign.center),
            ],
            const SizedBox(height: 16),
            FilledButton.icon(
              icon: const Icon(Icons.refresh),
              label: Text(actionLabel),
              onPressed: () => onAction(),
            ),
          ],
        ),
      ),
    );
  }
}
