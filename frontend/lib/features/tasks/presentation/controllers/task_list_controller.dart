import 'dart:async';
import 'dart:developer' show log;

import 'package:dio/dio.dart' show CancelToken;
import 'package:frontend/core/api/realtime_failure_mapper.dart';
import 'package:frontend/core/api/realtime_session_failure.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/utils/uuid.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/domain/task_list_filter_match.dart';
import 'package:frontend/features/tasks/domain/task_list_sort.dart';
import 'package:frontend/features/tasks/domain/task_model_to_list_item.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:meta/meta.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'task_list_controller.g.dart';

bool _hasMoreAfterTaskPage({
  required int nextOffset,
  required int total,
  required Set<String> idsBefore,
  required List<TaskListItemModel> pageTasks,
}) {
  if (pageTasks.isEmpty && nextOffset < total) {
    return false;
  }
  var newFromPage = 0;
  for (final t in pageTasks) {
    if (!idsBefore.contains(t.id)) {
      newFromPage++;
    }
  }
  final theoretical = nextOffset < total;
  return theoretical && newFromPage > 0;
}

List<TaskListItemModel> _mergeTaskItemsPage(
  List<TaskListItemModel> current,
  List<TaskListItemModel> incoming,
  TaskListFilter filter,
) {
  final byId = <String, TaskListItemModel>{};
  for (final t in current) {
    byId[t.id] = t;
  }
  for (final t in incoming) {
    byId[t.id] = t;
  }
  final out = byId.values.toList();
  sortTaskListItems(out, filter);
  return out;
}

bool _requiresConservativeListInvalidate(TaskListFilter filter) {
  final q = filter.search?.trim();
  if (q != null && q.isNotEmpty) {
    return true;
  }
  if (filter.parentTaskId != null && filter.parentTaskId!.isNotEmpty) {
    return true;
  }
  if (filter.rootOnly == true) {
    return true;
  }
  return false;
}

/// Список задач проекта: пагинация, фильтры, шов WS / мутаций (Sprint 12.3).
@Riverpod(keepAlive: true)
class TaskListController extends _$TaskListController {
  CancelToken? _listCancelToken;
  int _sessionEpoch = 0;
  Future<void>? _loadMoreInFlight;
  bool _wsRefetchInFlight = false;
  StreamSubscription<WsClientEvent>? _wsSubscription;

  String get _projectId => projectId;

  @override
  FutureOr<TaskListState> build({required String projectId}) {
    if (projectId.isEmpty || !isValidUuid(projectId)) {
      throw ArgumentError.value(
        projectId,
        'projectId',
        'must be non-empty valid UUID',
      );
    }

    _listCancelToken = CancelToken();
    // Один инстанс family + keepAlive: [build] один раз на жизненный цикл — гард на
    // `_wsSubscription` совпадает с «первая сборка», см. [ChatController.build] (11.9).
    if (_wsSubscription == null) {
      ref.onDispose(() {
        _listCancelToken?.cancel();
        unawaited(_wsSubscription?.cancel());
        _wsSubscription = null;
      });
      final ws = ref.read(webSocketServiceProvider);
      _wsSubscription = ws.events.listen(_onWsClientEvent);
      try {
        ws.connect(projectId);
      } on StateError catch (e, st) {
        log(
          'WS connect failed at TaskListController.build',
          name: 'TaskListController',
          error: e,
          stackTrace: st,
        );
        Future.microtask(() {
          if (!ref.mounted) {
            return;
          }
          applyRealtimeFailure(const WsServiceFailure.transient());
        });
      }
      Future.microtask(() {
        unawaited(_loadInitial());
      });
    }

    return TaskListState.initial();
  }

  bool _realtimeBlocksMutations() {
    final v = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    return v?.realtimeMutationBlocked == true;
  }

  /// Допускает мутации при [TaskListState.isLoadingInitial] во время [refresh] —
  /// параллельный `createTask` намеренно не блокируется (12.3); менять только осознанно.
  TaskMutationOutcome? _listMutationSurfaceGuard() {
    if (state is! AsyncData<TaskListState>) {
      return TaskMutationOutcome.notReady;
    }
    return null;
  }

  void _patchState(TaskListState Function(TaskListState s) fn) {
    final v = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (v == null) {
      return;
    }
    state = AsyncData(fn(v));
  }

  void _scheduleWsRefetch(Future<void> Function() job) {
    if (_wsRefetchInFlight) {
      return;
    }
    _wsRefetchInFlight = true;
    unawaited(
      job().whenComplete(() {
        _wsRefetchInFlight = false;
      }),
    );
  }

  /// REST refetch по сигналу WS (`needsRestRefetch`); антишторм (12.3 §59 / §64).
  void requestRestRefetch() {
    _scheduleWsRefetch(() => refresh(clearRealtimeBlocksOnSuccess: false));
  }

  /// Терминальный auth по [WsClientEventAuthFailure]; единая точка смены полей (12.9).
  void applyAuthFailure() {
    _patchState(
      (s) => s.copyWith(
        realtimeSessionFailure: const RealtimeSessionFailure.authenticationLost(),
        realtimeMutationBlocked: true,
        realtimeServiceFailure: null,
      ),
    );
  }

  void _clearRealtimeTransientFailure() {
    final v = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (v == null || v.realtimeServiceFailure == null) {
      return;
    }
    _patchState((s) => s.copyWith(realtimeServiceFailure: null));
  }

  void _onWsClientEvent(WsClientEvent ev) {
    switch (ev) {
      case WsClientEventServiceFailure(:final failure):
        applyRealtimeFailure(failure);
        return;
      case WsClientEventAuthFailure():
        applyAuthFailure();
        return;
      case WsClientEventSubprotocolMismatch():
      case WsClientEventParseError():
        _clearRealtimeTransientFailure();
        return;
      case WsClientEventServer(:final event):
        // Как [ChatController]: любой server-кадр сбрасывает transient до фильтра
        // `projectId` в payload — шум от другого проекта может преждевременно убрать
        // баннер (сознательно совпадаем с Chat, не KNOWN-ISSUE для 12.9).
        _clearRealtimeTransientFailure();
        event.when(
          taskStatus: applyWsTaskStatus,
          taskMessage: (_) {},
          agentLog: (_) {},
          error: (err) {
            if (err.projectId != _projectId) {
              return;
            }
            if (err.needsRestRefetch) {
              requestRestRefetch();
            }
          },
          unknown: (_) {},
        );
        return;
    }
  }

  void applyRealtimeFailure(WsServiceFailure failure) {
    final mapped = mapWsServiceFailureForTasks(failure);
    switch (mapped.kind) {
      case TaskRealtimeFailureKind.transient:
        _patchState(
          (s) => s.copyWith(realtimeServiceFailure: failure),
        );
      case TaskRealtimeFailureKind.terminalMutationBlock:
        _patchState(
          (s) => s.copyWith(
            realtimeSessionFailure: mapped.terminalSession,
            realtimeMutationBlocked: true,
            realtimeServiceFailure: null,
          ),
        );
    }
  }

  @visibleForTesting
  void setRealtimeMutationBlocked(bool value) {
    _patchState((s) => s.copyWith(realtimeMutationBlocked: value));
  }

  Future<void> refresh({bool clearRealtimeBlocksOnSuccess = true}) async {
    _sessionEpoch++;
    _listCancelToken?.cancel();
    _listCancelToken = CancelToken();

    final prev = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };

    state = AsyncData(
      TaskListState(
        filter: prev?.filter ?? TaskListFilter.defaults(),
        items: const [],
        total: 0,
        offset: 0,
        isLoadingInitial: true,
        isLoadingMore: false,
        hasMore: false,
        loadMoreError: null,
        realtimeMutationBlocked: prev?.realtimeMutationBlocked ?? false,
        realtimeSessionFailure: prev?.realtimeSessionFailure,
        realtimeServiceFailure: prev?.realtimeServiceFailure,
      ),
    );

    await _loadInitial(clearRealtimeBlocksOnSuccess: clearRealtimeBlocksOnSuccess);
  }

  Future<void> _loadInitial({bool clearRealtimeBlocksOnSuccess = false}) async {
    final epoch = _sessionEpoch;
    final token = _listCancelToken;
    if (token == null) {
      return;
    }

    final repo = ref.read(taskRepositoryProvider);
    final curFilter = switch (state) {
      AsyncData<TaskListState>(:final value) => value.filter,
      _ => TaskListFilter.defaults(),
    };

    try {
      final page = await repo.listTasks(
        _projectId,
        filter: curFilter,
        limit: kTaskListDefaultLimit,
        offset: 0,
        cancelToken: token,
      );
      if (epoch != _sessionEpoch) {
        return;
      }

      final merged = _mergeTaskItemsPage(const [], page.tasks, curFilter);
      final nextOffset = page.tasks.length;
      final idsBefore = <String>{};
      final hasMore = _hasMoreAfterTaskPage(
        nextOffset: nextOffset,
        total: page.total,
        idsBefore: idsBefore,
        pageTasks: page.tasks,
      );

      _patchState(
        (s) => s.copyWith(
          items: merged,
          total: page.total,
          offset: nextOffset,
          isLoadingInitial: false,
          hasMore: hasMore,
          loadMoreError: null,
          realtimeServiceFailure: null,
          realtimeSessionFailure:
              clearRealtimeBlocksOnSuccess ? null : s.realtimeSessionFailure,
          realtimeMutationBlocked:
              clearRealtimeBlocksOnSuccess ? false : s.realtimeMutationBlocked,
        ),
      );
    } on TaskCancelledException {
      if (epoch != _sessionEpoch) {
        return;
      }
    } catch (e, st) {
      if (epoch != _sessionEpoch) {
        return;
      }
      state = AsyncError(e, st);
    }
  }

  Future<void> loadMore() {
    final inflight = _loadMoreInFlight;
    if (inflight != null) {
      return inflight;
    }

    final s = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (s == null ||
        !s.hasMore ||
        s.isLoadingMore ||
        s.isLoadingInitial ||
        s.offset >= s.total) {
      return Future.value();
    }

    final epoch = _sessionEpoch;
    final token = _listCancelToken;
    if (token == null) {
      return Future.value();
    }

    late final Future<void> tracked;
    final f = _loadMoreImpl(epoch: epoch, token: token);
    tracked = f.whenComplete(() {
      if (identical(_loadMoreInFlight, tracked)) {
        _loadMoreInFlight = null;
      }
    });
    _loadMoreInFlight = tracked;
    return tracked;
  }

  Future<void> _loadMoreImpl({
    required int epoch,
    required CancelToken token,
  }) async {
    final cur = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (cur == null) {
      return;
    }

    _patchState((s) => s.copyWith(isLoadingMore: true, loadMoreError: null));

    final repo = ref.read(taskRepositoryProvider);
    try {
      final page = await repo.listTasks(
        _projectId,
        filter: cur.filter,
        limit: kTaskListDefaultLimit,
        offset: cur.offset,
        cancelToken: token,
      );
      if (epoch != _sessionEpoch) {
        return;
      }

      final after = switch (state) {
        AsyncData<TaskListState>(:final value) => value,
        _ => null,
      };
      if (after == null) {
        return;
      }

      final idsBefore = after.items.map((e) => e.id).toSet();
      final merged = _mergeTaskItemsPage(after.items, page.tasks, after.filter);
      final nextOffset = after.offset + page.tasks.length;
      final hasMore = _hasMoreAfterTaskPage(
        nextOffset: nextOffset,
        total: page.total,
        idsBefore: idsBefore,
        pageTasks: page.tasks,
      );

      _patchState(
        (s) => s.copyWith(
          items: merged,
          total: page.total,
          offset: nextOffset,
          isLoadingMore: false,
          hasMore: hasMore,
          loadMoreError: null,
        ),
      );
    } on TaskCancelledException {
      if (epoch == _sessionEpoch) {
        _patchState((s) => s.copyWith(isLoadingMore: false));
      }
    } catch (e) {
      if (epoch != _sessionEpoch) {
        return;
      }
      _patchState((s) => s.copyWith(isLoadingMore: false, loadMoreError: e));
    }
  }

  /// Смена фильтра: сброс страницы и перезагрузка.
  Future<void> setFilter(TaskListFilter next) async {
    final cur = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (cur == null) {
      return;
    }
    if (cur.filter == next) {
      return;
    }

    _sessionEpoch++;
    _listCancelToken?.cancel();
    _listCancelToken = CancelToken();

    state = AsyncData(
      cur.copyWith(
        filter: next,
        items: const [],
        offset: 0,
        total: 0,
        hasMore: false,
        isLoadingInitial: true,
        isLoadingMore: false,
        loadMoreError: null,
      ),
    );

    await _loadInitial();
  }

  Future<TaskMutationOutcome> createTask(CreateTaskRequest request) async {
    final guard = _listMutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    final repo = ref.read(taskRepositoryProvider);
    await repo.createTask(_projectId, request, cancelToken: null);
    await refresh();
    return TaskMutationOutcome.completed;
  }

  /// После HTTP-мутации из деталей (12.3 кросс-контроллер).
  void syncListFromHttpTask(TaskModel model) {
    if (model.projectId != _projectId) {
      return;
    }

    final s = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (s == null) {
      return;
    }

    if (_requiresConservativeListInvalidate(s.filter)) {
      unawaited(refresh());
      return;
    }

    final item = taskModelToListItem(model);
    if (!taskListItemMatchesCurrentFilter(item, s.filter)) {
      _patchState((prev) {
        final nextItems =
            prev.items.where((e) => e.id != model.id).toList(growable: false);
        return prev.copyWith(items: nextItems);
      });
      return;
    }

    final hadRow = s.items.any((e) => e.id == item.id);
    if (!hadRow && s.total > s.items.length) {
      unawaited(refresh());
      return;
    }

    _patchState((prev) {
      final map = {for (final t in prev.items) t.id: t};
      map[item.id] = item;
      final out = map.values.toList();
      sortTaskListItems(out, prev.filter);
      return prev.copyWith(items: out);
    });
  }

  void applyWsTaskStatus(WsTaskStatusEvent e) {
    if (e.projectId != _projectId || e.taskId.isEmpty) {
      return;
    }

    final s = switch (state) {
      AsyncData<TaskListState>(:final value) => value,
      _ => null,
    };
    if (s == null) {
      return;
    }

    final idx = s.items.indexWhere((it) => it.id == e.taskId);
    if (idx < 0) {
      return;
    }

    final row = s.items[idx];
    if (row.status != e.previousStatus) {
      _refetchListRowFromWs(e.taskId);
      return;
    }

    final agentId = e.assignedAgentId;
    final agentChanged = agentId != null &&
        agentId.isNotEmpty &&
        agentId != row.assignedAgent?.id;

    if (agentChanged) {
      _refetchListRowFromWs(e.taskId);
      return;
    }

    final patched = row.copyWith(status: e.status);
    _patchState((prev) {
      final i = prev.items.indexWhere((it) => it.id == e.taskId);
      if (i < 0) {
        return prev;
      }
      final next = List<TaskListItemModel>.from(prev.items);
      next[i] = patched;
      sortTaskListItems(next, prev.filter);
      return prev.copyWith(items: next);
    });
  }

  void _refetchListRowFromWs(String taskId) {
    _scheduleWsRefetch(() async {
      final repo = ref.read(taskRepositoryProvider);
      final model = await repo.getTask(taskId, cancelToken: null);
      final fresh = taskModelToListItem(model);
      _patchState((prev) {
        final i = prev.items.indexWhere((it) => it.id == taskId);
        if (i < 0) {
          return prev;
        }
        final next = List<TaskListItemModel>.from(prev.items);
        next[i] = fresh;
        sortTaskListItems(next, prev.filter);
        return prev.copyWith(items: next);
      });
    });
  }
}
