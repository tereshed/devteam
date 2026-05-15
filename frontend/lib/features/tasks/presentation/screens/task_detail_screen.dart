import 'dart:async';
import 'dart:convert';

import 'package:flutter/foundation.dart' show mapEquals;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/domain/models/task_message_model.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/features/tasks/presentation/utils/task_message_display.dart';
import 'package:frontend/features/tasks/presentation/utils/task_message_metadata_redaction.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifacts_dag_section.dart';
import 'package:frontend/features/tasks/presentation/widgets/router_timeline_section.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_timeout_editor.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/shared/widgets/diff_viewer.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

/// Порог ширины: RefreshIndicator vs AppBar refresh (12.4 / 12.5).
const double kTaskDetailMobileBreakpointWidth = 600;

/// Триггер догрузки сообщений у нижнего края ленты (12.5).
const int kTaskDetailMessageLoadMoreTrailingThreshold = 3;

/// Key: баннер ошибки догрузки сообщений (виджет-тесты 12.5).
const ValueKey<String> kTaskDetailMessagesLoadMoreErrorBannerKey =
    ValueKey<String>('task_detail_messages_load_more_error_banner');

bool _taskDetailShowCancelForStatus(String status) {
  return status == 'active' || status == 'needs_human';
}

/// Панель lifecycle только если есть хотя бы одно действие (12.8; неизвестный статус — без пустого отступа).
bool taskDetailLifecyclePanelVisibleForStatus(String status) {
  return _taskDetailShowCancelForStatus(status);
}

class _LifecycleActionRow {
  const _LifecycleActionRow({
    required this.visible,
    required this.busy,
    required this.label,
    required this.icon,
    required this.onPressed,
  });

  final bool visible;
  final bool busy;
  final String label;
  final IconData icon;
  final VoidCallback? onPressed;
}

List<_LifecycleActionRow> _taskDetailLifecycleActionRows(
  AppLocalizations l10n,
  TaskDetailState data, {
  required VoidCallback onCancel,
}) {
  final status = data.task!.status;
  final rt = data.realtimeMutationBlocked;
  final inflight = data.lifecycleMutationInFlight;
  final canPress = !rt && inflight == null;

  return [
    _LifecycleActionRow(
      visible: _taskDetailShowCancelForStatus(status),
      busy: inflight == TaskLifecycleMutation.cancel,
      label: l10n.taskActionCancel,
      icon: Icons.cancel_outlined,
      onPressed: canPress ? onCancel : null,
    ),
  ];
}

/// Скрыть pull-to-refresh / иконку обновления при удалённой задаче или mismatch проекта.
bool _hideTaskDetailRefresh(AsyncValue<TaskDetailState> async) {
  final mismatch =
      async.hasError && async.error is TaskDetailProjectMismatchException;
  final deleted = async.maybeWhen(
    data: (d) => d.taskDeleted,
    orElse: () => false,
  );
  return deleted || mismatch;
}

/// Экран деталей задачи: статус, описание, лог, diff, результат (12.5).
class TaskDetailScreen extends ConsumerStatefulWidget {
  const TaskDetailScreen({
    super.key,
    required this.projectId,
    required this.taskId,
  });

  final String projectId;
  final String taskId;

  @override
  ConsumerState<TaskDetailScreen> createState() => _TaskDetailScreenState();
}

class _TaskDetailScreenState extends ConsumerState<TaskDetailScreen> {
  static const Duration _kRefreshTimeout = Duration(seconds: 30);

  late final ScrollController _scrollController = ScrollController();
  bool _didInitialScrollToBottom = false;
  /// Один повторный postFrame, если ScrollController ещё без клиентов (избегаем бесконечной цепочки).
  int _initialScrollAttachRetries = 0;

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  @override
  void didUpdateWidget(TaskDetailScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.projectId != widget.projectId ||
        oldWidget.taskId != widget.taskId) {
      _didInitialScrollToBottom = false;
      _initialScrollAttachRetries = 0;
    }
  }

  Future<void> _onRefresh() async {
    final messenger = ScaffoldMessenger.of(context);
    final l10n = AppLocalizations.of(context)!;
    try {
      await ref
          .read(
            taskDetailControllerProvider(
              projectId: widget.projectId,
              taskId: widget.taskId,
            ).notifier,
          )
          .refresh()
          .timeout(_kRefreshTimeout);
    } on TimeoutException {
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.taskDetailRefreshTimedOut)),
      );
    }
  }

  void _maybeScrollToBottomOnce(TaskDetailState data) {
    if (_didInitialScrollToBottom ||
        data.taskDeleted ||
        data.messages.isEmpty) {
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      final async = ref.read(
        taskDetailControllerProvider(
          projectId: widget.projectId,
          taskId: widget.taskId,
        ),
      );
      final cur = switch (async) {
        AsyncData<TaskDetailState>(:final value) => value,
        _ => null,
      };
      if (cur == null || cur.taskDeleted || cur.messages.isEmpty) {
        return;
      }
      if (!_scrollController.hasClients) {
        if (_initialScrollAttachRetries >= 1) {
          return;
        }
        _initialScrollAttachRetries++;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) {
            return;
          }
          _maybeScrollToBottomOnce(cur);
        });
        return;
      }
      _scrollController.jumpTo(_scrollController.position.maxScrollExtent);
      _didInitialScrollToBottom = true;
    });
  }

  void _scheduleLoadMoreIfNeeded({
    required int index,
    required int messageCount,
    required TaskDetailState data,
  }) {
    if (messageCount == 0) {
      return;
    }
    const threshold = kTaskDetailMessageLoadMoreTrailingThreshold;
    final tailStart = messageCount <= threshold ? 0 : messageCount - threshold;
    if (index < tailStart) {
      return;
    }
    if (!data.hasMoreMessages || data.isLoadingMessages) {
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      unawaited(
        ref
            .read(
              taskDetailControllerProvider(
                projectId: widget.projectId,
                taskId: widget.taskId,
              ).notifier,
            )
            .loadMoreMessages(),
      );
    });
  }

  Future<void> _applyMessageTypeFilter(String? selected) async {
    final prov = taskDetailControllerProvider(
      projectId: widget.projectId,
      taskId: widget.taskId,
    );
    final async = ref.read(prov);
    final current = switch (async) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (current == null) {
      return;
    }
    if (selected == current.messageTypeFilter) {
      return;
    }
    await ref.read(prov.notifier).setMessageFilters(
          messageType: selected,
          senderType: current.senderTypeFilter,
        );
  }

  Future<void> _applySenderTypeFilter(String? selected) async {
    final prov = taskDetailControllerProvider(
      projectId: widget.projectId,
      taskId: widget.taskId,
    );
    final async = ref.read(prov);
    final current = switch (async) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (current == null) {
      return;
    }
    if (selected == current.senderTypeFilter) {
      return;
    }
    await ref.read(prov.notifier).setMessageFilters(
          messageType: current.messageTypeFilter,
          senderType: selected,
        );
  }

  TaskDetailController _taskDetailNotifier() => ref.read(
        taskDetailControllerProvider(
          projectId: widget.projectId,
          taskId: widget.taskId,
        ).notifier,
      );

  Future<void> _applyLifecycleMutation(
    Future<TaskMutationOutcome> Function(TaskDetailController n) call,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final messenger = ScaffoldMessenger.of(context);
    try {
      final o = await call(_taskDetailNotifier());
      if (!mounted) {
        return;
      }
      if (o == TaskMutationOutcome.blockedByRealtime) {
        messenger.showSnackBar(
          SnackBar(content: Text(l10n.taskActionBlockedByRealtimeSnack)),
        );
      } else if (o == TaskMutationOutcome.alreadyTerminal) {
        // Race: задача уже завершена — info-toast, не красный snack.
        messenger.showSnackBar(
          SnackBar(content: Text(l10n.taskActionAlreadyTerminalSnack)),
        );
      }
    } catch (_) {
      // [TaskDetailController] выставляет AsyncError с copyWithPrevious; snack — через [ref.listen].
    }
  }

  Future<void> _onCancelPressed() async {
    final l10n = AppLocalizations.of(context)!;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.taskActionCancelConfirmTitle),
        content: Text(l10n.taskActionCancelConfirmBody),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.taskActionConfirm),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) {
      return;
    }
    await _applyLifecycleMutation((n) => n.cancelTask());
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final async = ref.watch(
      taskDetailControllerProvider(
        projectId: widget.projectId,
        taskId: widget.taskId,
      ),
    );

    ref.listen(
      taskDetailControllerProvider(
        projectId: widget.projectId,
        taskId: widget.taskId,
      ),
      (prev, next) {
        next.whenOrNull(
          data: (data) => _maybeScrollToBottomOnce(data),
        );
        if (next.hasError && next.hasValue && context.mounted) {
          final err = next.error!;
          if (err is TaskDetailProjectMismatchException) {
            return;
          }
          // [_patchState] переиспользует тот же объект ошибки — без этого снек
          // дублируется на каждое WS-обновление (ит.4 фикс №2).
          if (prev != null &&
              prev.hasError &&
              identical(prev.error, err)) {
            return;
          }
          final detail = taskErrorDetail(err);
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(taskDetailErrorTitle(l10n, err)),
                  if (detail != null) Text(detail),
                ],
              ),
              action: SnackBarAction(
                label: l10n.retry,
                onPressed: () => unawaited(_onRefresh()),
              ),
            ),
          );
        }
      },
    );

    final width = MediaQuery.sizeOf(context).width;
    final isWide = width >= kTaskDetailMobileBreakpointWidth;

    final titleWidget = async.hasError &&
            async.hasValue &&
            (async.requireValue.task != null || async.requireValue.taskDeleted)
        ? _appBarTitleForDetailState(l10n, async.requireValue)
        : async.when(
            data: (d) => _appBarTitleForDetailState(l10n, d),
            error: (e, _) => Text(
              e is TaskDetailProjectMismatchException
                  ? l10n.taskDetailProjectMismatch
                  : taskDetailErrorTitle(l10n, e),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
            loading: () => Text(l10n.taskDetailAppBarLoading),
          );

    final lifecycleAppBarIcons = <Widget>[
      if (isWide)
        ...switch (async) {
          AsyncData<TaskDetailState>(:final value) =>
            (!value.taskDeleted &&
                    value.task != null &&
                    taskDetailLifecyclePanelVisibleForStatus(
                      value.task!.status,
                    ))
                ? _taskDetailLifecycleAppBarActions(
                    l10n,
                    value,
                    onCancel: () => unawaited(_onCancelPressed()),
                  )
                : const <Widget>[],
          _ => const <Widget>[],
        },
    ];

    return Scaffold(
      appBar: AppBar(
        // TODO(12.5 web/PWA): при прямом открытии URL без стека Navigator.canPop == false —
        // рассмотреть fallback context.go(/projects/:id/tasks) вместо no-op (канон спеки — BackButton).
        leading: const BackButton(),
        title: titleWidget,
        actions: [
          ...lifecycleAppBarIcons,
          if (isWide && !_hideTaskDetailRefresh(async))
            IconButton(
              tooltip: MaterialLocalizations.of(context)
                  .refreshIndicatorSemanticLabel,
              onPressed: () => unawaited(_onRefresh()),
              icon: const Icon(Icons.refresh),
            ),
        ],
      ),
      body: async.when(
        data: (data) => _scrollableTaskDetailBody(
          context: context,
          l10n: l10n,
          data: data,
          isWide: isWide,
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) {
          if (e is TaskDetailProjectMismatchException) {
            return _DeletedOrMismatchBody(
              projectId: widget.projectId,
              message: l10n.taskDetailProjectMismatch,
            );
          }
          if (async.hasValue) {
            final preserved = async.requireValue;
            if (preserved.task != null || preserved.taskDeleted) {
              return _scrollableTaskDetailBody(
                context: context,
                l10n: l10n,
                data: preserved,
                isWide: isWide,
              );
            }
          }
          final detail = taskErrorDetail(e);
          return Center(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    Icons.error_outline,
                    size: 48,
                    color: Theme.of(context).colorScheme.error,
                  ),
                  const SizedBox(height: 12),
                  Text(taskDetailErrorTitle(l10n, e), textAlign: TextAlign.center),
                  if (detail != null) ...[
                    const SizedBox(height: 8),
                    Text(detail, textAlign: TextAlign.center),
                  ],
                  const SizedBox(height: 16),
                  FilledButton.icon(
                    onPressed: () => unawaited(_onRefresh()),
                    icon: const Icon(Icons.refresh),
                    label: Text(l10n.retry),
                  ),
                  const SizedBox(height: 8),
                  OutlinedButton(
                    onPressed: () => context.go('/projects/${widget.projectId}/tasks'),
                    child: Text(l10n.taskDetailBackToList),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _appBarTitleForDetailState(
    AppLocalizations l10n,
    TaskDetailState d,
  ) {
    if (d.taskDeleted) {
      return Text(l10n.taskDetailDeletedTitle);
    }
    final t = d.task?.title;
    if (t != null && t.isNotEmpty) {
      return Text(
        t,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
      );
    }
    return Text(l10n.taskDetailAppBarLoading);
  }

  Widget _scrollableTaskDetailBody({
    required BuildContext context,
    required AppLocalizations l10n,
    required TaskDetailState data,
    required bool isWide,
  }) {
    if (data.taskDeleted) {
      return _DeletedOrMismatchBody(
        projectId: widget.projectId,
        message: l10n.taskDetailDeletedBody,
      );
    }
    if (data.task == null) {
      return const Center(child: CircularProgressIndicator());
    }
    final scrollView = CustomScrollView(
      controller: _scrollController,
      physics: const AlwaysScrollableScrollPhysics(),
      slivers: [
        ..._realtimeSlivers(context, l10n, data),
        SliverToBoxAdapter(
          child: _TaskHeaderSection(
            projectId: widget.projectId,
            taskId: widget.taskId,
            l10n: l10n,
            data: data,
          ),
        ),
        if (!isWide &&
            data.task != null &&
            taskDetailLifecyclePanelVisibleForStatus(data.task!.status))
          SliverToBoxAdapter(
            child: _TaskLifecycleMobileActions(
              l10n: l10n,
              data: data,
              onCancel: () => unawaited(_onCancelPressed()),
            ),
          ),
        SliverToBoxAdapter(
          child: _SectionBlock(
            title: l10n.taskDetailSectionDescription,
            child: _descriptionBody(context, l10n, data),
          ),
        ),
        if (_hasErrorMessage(data))
          SliverToBoxAdapter(
            child: _SectionBlock(
              title: l10n.taskDetailSectionErrorMessage,
              child: Text(
                data.task!.errorMessage!.trim(),
                style: TextStyle(color: Theme.of(context).colorScheme.error),
              ),
            ),
          ),
        SliverToBoxAdapter(
          child: _SectionBlock(
            title: l10n.taskDetailSectionResult,
            child: _resultBody(context, l10n, data),
          ),
        ),
        SliverToBoxAdapter(
          child: _SectionBlock(
            title: l10n.taskDetailSectionDiff,
            child: _diffBody(context, l10n, data),
          ),
        ),
        SliverToBoxAdapter(
          child: _SubtasksSection(
            projectId: widget.projectId,
            l10n: l10n,
            data: data,
          ),
        ),
        SliverToBoxAdapter(
          child: _SectionBlock(
            title: l10n.artifactsSection,
            child: ArtifactsDagSection(taskId: widget.taskId),
          ),
        ),
        SliverToBoxAdapter(
          child: _SectionBlock(
            title: l10n.routerTimelineSection,
            child: RouterTimelineSection(taskId: widget.taskId),
          ),
        ),
        SliverToBoxAdapter(
          child: _MessageFiltersBar(
            l10n: l10n,
            data: data,
            onMessageType: (v) async {
              await _applyMessageTypeFilter(v);
            },
            onSenderType: (v) async {
              await _applySenderTypeFilter(v);
            },
          ),
        ),
        if (data.messagesLoadMoreError != null)
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
              child: _MessagesLoadMoreErrorBanner(
                l10n: l10n,
                error: data.messagesLoadMoreError!,
                onRetry: () => unawaited(
                  ref
                      .read(
                        taskDetailControllerProvider(
                          projectId: widget.projectId,
                          taskId: widget.taskId,
                        ).notifier,
                      )
                      .retryMessagesAfterError(),
                ),
              ),
            ),
          ),
        if (data.messages.isEmpty && data.isLoadingMessages)
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CircularProgressIndicator()),
            ),
          )
        else if (data.messages.isEmpty)
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: Text(
                l10n.taskDetailNoMessages,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
            ),
          )
        else
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            sliver: SliverList(
              // Future-work (12.9+): при вставках не в конец — добавить findChildIndexCallback.
              delegate: SliverChildBuilderDelegate(
                (context, index) {
                  _scheduleLoadMoreIfNeeded(
                    index: index,
                    messageCount: data.messages.length,
                    data: data,
                  );
                  final msg = data.messages[index];
                  return RepaintBoundary(
                    child: _TaskMessageTile(
                      l10n: l10n,
                      message: msg,
                    ),
                  );
                },
                childCount: data.messages.length,
              ),
            ),
          ),
        if (data.isLoadingMessages &&
            data.hasMoreMessages &&
            data.messages.isNotEmpty)
          const SliverToBoxAdapter(
            child: Padding(
              padding: EdgeInsets.all(16),
              child: Center(child: CircularProgressIndicator()),
            ),
          ),
        const SliverToBoxAdapter(child: SizedBox(height: 24)),
      ],
    );
    if (isWide) {
      return scrollView;
    }
    return RefreshIndicator(
      onRefresh: _onRefresh,
      child: scrollView,
    );
  }

  bool _hasErrorMessage(TaskDetailState data) {
    final em = data.task?.errorMessage;
    return em != null && em.trim().isNotEmpty;
  }

  Widget _descriptionBody(
    BuildContext context,
    AppLocalizations l10n,
    TaskDetailState data,
  ) {
    final d = data.task?.description ?? '';
    if (d.trim().isEmpty) {
      return Text(
        l10n.taskDetailNoDescription,
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.outline,
            ),
      );
    }
    return SelectableText(d, style: Theme.of(context).textTheme.bodyMedium);
  }

  Widget _resultBody(
    BuildContext context,
    AppLocalizations l10n,
    TaskDetailState data,
  ) {
    final r = data.task?.result;
    if (r == null || r.trim().isEmpty) {
      return Text(
        l10n.taskDetailNoResult,
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.outline,
            ),
      );
    }
    return SelectableText(r, style: Theme.of(context).textTheme.bodyMedium);
  }

  Widget _diffBody(
    BuildContext context,
    AppLocalizations l10n,
    TaskDetailState data,
  ) {
    final raw = data.task?.artifacts['diff'];
    final s = raw is String ? raw : null;
    if (s == null || s.trim().isEmpty) {
      return Text(
        l10n.taskDetailNoDiff,
        style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.outline,
            ),
      );
    }
    return DiffViewer(diff: s);
  }

  List<Widget> _realtimeSlivers(
    BuildContext context,
    AppLocalizations l10n,
    TaskDetailState data,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final out = <Widget>[];
    if (data.realtimeMutationBlocked) {
      out.add(
        SliverToBoxAdapter(
          child: _BannerStrip(
            color: scheme.errorContainer,
            onColor: scheme.onErrorContainer,
            text: l10n.taskDetailRealtimeMutationBlocked,
          ),
        ),
      );
    }
    if (data.realtimeSessionFailure != null) {
      out.add(
        SliverToBoxAdapter(
          child: _BannerStrip(
            color: scheme.errorContainer,
            onColor: scheme.onErrorContainer,
            text: l10n.taskDetailRealtimeSessionFailure,
          ),
        ),
      );
    }
    if (data.realtimeServiceFailure != null) {
      out.add(
        SliverToBoxAdapter(
          child: _BannerStrip(
            color: scheme.secondaryContainer,
            onColor: scheme.onSecondaryContainer,
            text: l10n.taskDetailRealtimeServiceFailure,
          ),
        ),
      );
    }
    return out;
  }
}

List<Widget> _taskDetailLifecycleAppBarActions(
  AppLocalizations l10n,
  TaskDetailState data, {
  required VoidCallback onCancel,
}) {
  final rows = _taskDetailLifecycleActionRows(
    l10n,
    data,
    onCancel: onCancel,
  );
  return [
    for (final r in rows)
      if (r.visible)
        IconButton(
          tooltip: r.label,
          onPressed: r.onPressed,
          icon: r.busy
              ? const SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Icon(r.icon),
        ),
  ];
}

class _TaskLifecycleMobileActions extends StatelessWidget {
  const _TaskLifecycleMobileActions({
    required this.l10n,
    required this.data,
    required this.onCancel,
  });

  final AppLocalizations l10n;
  final TaskDetailState data;
  final VoidCallback onCancel;

  @override
  Widget build(BuildContext context) {
    final rows = _taskDetailLifecycleActionRows(
      l10n,
      data,
      onCancel: onCancel,
    );

    return Padding(
      key: const ValueKey<String>('task_detail_lifecycle_mobile'),
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          for (final r in rows)
            if (r.visible)
              _taskLifecycleMobileButton(
                label: r.label,
                icon: r.icon,
                isBusy: r.busy,
                onPressed: r.onPressed,
              ),
        ],
      ),
    );
  }
}

Widget _taskLifecycleMobileButton({
  required String label,
  required IconData icon,
  required bool isBusy,
  required VoidCallback? onPressed,
}) {
  return FilledButton.tonalIcon(
    onPressed: onPressed,
    icon: isBusy
        ? const SizedBox(
            width: 18,
            height: 18,
            child: CircularProgressIndicator(strokeWidth: 2),
          )
        : Icon(icon, size: 20),
    label: Text(label),
  );
}

class _BannerStrip extends StatelessWidget {
  const _BannerStrip({
    required this.color,
    required this.onColor,
    required this.text,
  });

  final Color color;
  final Color onColor;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: color,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
        child: Semantics(
          liveRegion: true,
          child: Text(
            text,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: onColor),
          ),
        ),
      ),
    );
  }
}

class _DeletedOrMismatchBody extends StatelessWidget {
  const _DeletedOrMismatchBody({
    required this.projectId,
    required this.message,
  });

  final String projectId;
  final String message;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Padding(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(message, style: Theme.of(context).textTheme.bodyLarge),
          const SizedBox(height: 24),
          FilledButton(
            onPressed: () => context.go('/projects/$projectId/tasks'),
            child: Text(l10n.taskDetailBackToList),
          ),
        ],
      ),
    );
  }
}

class _TaskHeaderSection extends ConsumerWidget {
  const _TaskHeaderSection({
    required this.projectId,
    required this.taskId,
    required this.l10n,
    required this.data,
  });

  final String projectId;
  final String taskId;
  final AppLocalizations l10n;
  final TaskDetailState data;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final task = data.task;
    if (task == null && data.isLoadingTask) {
      return const Padding(
        padding: EdgeInsets.all(24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (task == null) {
      return const SizedBox.shrink();
    }
    final scheme = Theme.of(context).colorScheme;
    final stTone = taskStatusTone(task.status);
    final prTone = taskPriorityTone(task.priority);
    final agent = task.assignedAgent;
    final hasOverride =
        task.customTimeout != null && task.customTimeout!.isNotEmpty;
    final timeoutDisabled = data.realtimeMutationBlocked ||
        data.lifecycleMutationInFlight != null;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              Chip(
                avatar: Icon(taskStatusIcon(stTone), size: 18),
                label: Text(taskStatusLabel(l10n, task.status)),
                backgroundColor: taskStatusChipBackground(scheme, stTone),
                labelStyle: TextStyle(color: taskStatusChipForeground(scheme, stTone)),
              ),
              Chip(
                avatar: Icon(taskPriorityIcon(prTone), size: 18),
                label: Text(taskPriorityLabel(l10n, task.priority)),
                backgroundColor: taskPriorityChipBackground(scheme, prTone),
                labelStyle: TextStyle(color: taskPriorityChipForeground(scheme, prTone)),
              ),
              InputChip(
                avatar: Icon(
                  hasOverride ? Icons.timer : Icons.timer_outlined,
                  size: 18,
                ),
                label: Text(
                  hasOverride
                      ? '${l10n.tasksCustomTimeoutSectionTitle}: ${task.customTimeout}'
                      : '${l10n.tasksCustomTimeoutSectionTitle}: ${l10n.tasksCustomTimeoutNone}',
                ),
                backgroundColor: hasOverride
                    ? scheme.secondaryContainer
                    : scheme.surfaceContainerHighest,
                labelStyle: TextStyle(
                  color: hasOverride
                      ? scheme.onSecondaryContainer
                      : scheme.onSurfaceVariant,
                ),
                tooltip: l10n.tasksCustomTimeoutEdit,
                onPressed: timeoutDisabled
                    ? null
                    : () => unawaited(showTaskTimeoutDialog(
                          context: context,
                          ref: ref,
                          projectId: projectId,
                          taskId: taskId,
                          currentValue: task.customTimeout,
                        )),
              ),
            ],
          ),
          if (agent != null) ...[
            const SizedBox(height: 12),
            Text(
              agent.name,
              style: Theme.of(context).textTheme.titleMedium,
            ),
            Text(
              agentRoleLabel(l10n, agent.role),
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
          ],
        ],
      ),
    );
  }
}

class _SectionBlock extends StatelessWidget {
  const _SectionBlock({
    required this.title,
    required this.child,
  });

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            title,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          child,
        ],
      ),
    );
  }
}

class _SubtasksSection extends StatelessWidget {
  const _SubtasksSection({
    required this.projectId,
    required this.l10n,
    required this.data,
  });

  final String projectId;
  final AppLocalizations l10n;
  final TaskDetailState data;

  // Future-work: при очень большом числе подзадач рассмотреть ленивый список (12.5 review).

  @override
  Widget build(BuildContext context) {
    final subs = data.task?.subTasks ?? const [];
    if (subs.isEmpty) {
      return const SizedBox.shrink();
    }
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            l10n.taskDetailSectionSubtasks,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          ...subs.map(
            (s) => ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(s.title),
              subtitle: Text(taskStatusLabel(l10n, s.status)),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => context.pushReplacement(
                '/projects/$projectId/tasks/${s.id}',
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _MessageFiltersBar extends StatelessWidget {
  const _MessageFiltersBar({
    required this.l10n,
    required this.data,
    required this.onMessageType,
    required this.onSenderType,
  });

  final AppLocalizations l10n;
  final TaskDetailState data;
  final Future<void> Function(String?) onMessageType;
  final Future<void> Function(String?) onSenderType;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            l10n.taskDetailSectionMessages,
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 8),
          SizedBox(
            height: 40,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: messageTypes.length + 1,
              separatorBuilder: (_, _) => const SizedBox(width: 8),
              itemBuilder: (context, i) {
                final sel = i == 0 ? null : messageTypes[i - 1];
                final selected = data.messageTypeFilter == sel;
                final label =
                    i == 0 ? l10n.filterAll : taskMessageTypeLabel(l10n, sel!);
                return FilterChip(
                  label: Text(label),
                  selected: selected,
                  onSelected: (_) {
                    unawaited(onMessageType(sel));
                  },
                );
              },
            ),
          ),
          const SizedBox(height: 8),
          SizedBox(
            height: 40,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: senderTypes.length + 1,
              separatorBuilder: (_, _) => const SizedBox(width: 8),
              itemBuilder: (context, i) {
                final sel = i == 0 ? null : senderTypes[i - 1];
                final selected = data.senderTypeFilter == sel;
                final label =
                    i == 0 ? l10n.filterAll : taskSenderTypeLabel(l10n, sel!);
                return FilterChip(
                  label: Text(label),
                  selected: selected,
                  onSelected: (_) {
                    unawaited(onSenderType(sel));
                  },
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _TaskMessageTile extends StatefulWidget {
  const _TaskMessageTile({
    required this.l10n,
    required this.message,
  });

  final AppLocalizations l10n;
  final TaskMessageModel message;

  @override
  State<_TaskMessageTile> createState() => _TaskMessageTileState();
}

class _TaskMessageTileState extends State<_TaskMessageTile> {
  late String? _cachedMetaStr;
  late String _formattedTs;

  @override
  void initState() {
    super.initState();
    _cachedMetaStr = _computeMetaStr(widget.message);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _refreshFormattedTs();
  }

  @override
  void didUpdateWidget(covariant _TaskMessageTile oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.message.id != widget.message.id ||
        !mapEquals(oldWidget.message.metadata, widget.message.metadata)) {
      _cachedMetaStr = _computeMetaStr(widget.message);
    }
    if (oldWidget.message.createdAt != widget.message.createdAt) {
      _refreshFormattedTs();
    }
  }

  void _refreshFormattedTs() {
    final localeTag = Localizations.localeOf(context).toLanguageTag();
    _formattedTs = DateFormat.yMMMd(localeTag).add_jm().format(
          widget.message.createdAt.toLocal(),
        );
  }

  String? _computeMetaStr(TaskMessageModel m) {
    if (m.metadata.isEmpty) {
      return null;
    }
    final meta = redactTaskMessageMetadata(m.metadata);
    return const JsonEncoder.withIndent('  ').convert(meta);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final metaStr = _cachedMetaStr;

    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerLow,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                '${taskSenderTypeLabel(widget.l10n, widget.message.senderType)} · '
                '${taskMessageTypeLabel(widget.l10n, widget.message.messageType)} · '
                '$_formattedTs',
                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
              ),
              const SizedBox(height: 6),
              SelectableText(
                widget.message.content,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              if (metaStr != null) ...[
                const SizedBox(height: 8),
                SelectableText(
                  metaStr,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        fontFamily: 'monospace',
                        color: scheme.outline,
                      ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _MessagesLoadMoreErrorBanner extends StatelessWidget {
  const _MessagesLoadMoreErrorBanner({
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
      key: kTaskDetailMessagesLoadMoreErrorBannerKey,
      color: scheme.errorContainer,
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              taskDetailErrorTitle(l10n, error),
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
