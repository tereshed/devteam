import 'dart:async';
import 'dart:convert' show utf8;
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
import 'package:frontend/features/tasks/domain/task_messages_merge.dart';
import 'package:frontend/features/tasks/domain/ws_task_message_mapper.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:meta/meta.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'task_detail_controller.g.dart';

bool _hasMoreMessagePage({
  required int nextOffset,
  required int total,
  required Set<String> idsBefore,
  required List<TaskMessageModel> page,
}) {
  if (page.isEmpty && nextOffset < total) {
    return false;
  }
  var newFromPage = 0;
  for (final m in page) {
    if (!idsBefore.contains(m.id)) {
      newFromPage++;
    }
  }
  final theoretical = nextOffset < total;
  return theoretical && newFromPage > 0;
}

/// Слияние карточки после REST: принимаем [incoming] только если его `updatedAt`
/// **строго** новее — при равенстве дат оставляем [cur], чтобы не откатывать
/// WS-патч статуса при отстающем COMMIT в БД (12.3 § свежесть).
TaskModel _pickFresherTask(TaskModel? cur, TaskModel incoming) {
  if (cur == null) {
    return incoming;
  }
  return incoming.updatedAt.isAfter(cur.updatedAt) ? incoming : cur;
}

/// Детали задачи и сообщения (12.3). Провайдер **autoDispose** — без `keepAlive: true`.
@Riverpod()
class TaskDetailController extends _$TaskDetailController {
  CancelToken? _taskHistoryToken;
  CancelToken? _messagesHistoryToken;

  int _taskEpoch = 0;
  int _messagesEpoch = 0;

  /// Инвалидирует in-flight lifecycle POST после [refresh] / смены «поколения» UI,
  /// чтобы поздний ответ не затирал актуальное состояние и не выставлял [AsyncError].
  int _lifecycleEpoch = 0;

  Future<void>? _loadMoreMessagesInFlight;

  /// Один guard на все WS-инициированные REST-ходы (12.3 §64): reconcile при FSM mismatch,
  /// [requestRestRefetch], фоновый `getTask` после смены агента. Параллельный второй запрос
  /// отбрасывается до [whenComplete] текущего; это сознательный trade-off (например, смена
  /// агента во время reconcile не ставится в очередь — следующее WS-событие или ручной refresh
  /// подтянут карточку).
  bool _wsRefetchInFlight = false;

  StreamSubscription<WsClientEvent>? _wsSubscription;

  String get _projectId => projectId;

  String get _taskId => taskId;

  @override
  FutureOr<TaskDetailState> build({
    required String projectId,
    required String taskId,
  }) {
    if (projectId.isEmpty ||
        taskId.isEmpty ||
        !isValidUuid(projectId) ||
        !isValidUuid(taskId)) {
      throw ArgumentError(
        'projectId and taskId must be non-empty valid UUIDs',
      );
    }

    _taskHistoryToken = CancelToken();
    _messagesHistoryToken = CancelToken();
    // Family autoDispose: [build] один раз на инстанс — гард совпадает с первой подпиской (11.9).
    if (_wsSubscription == null) {
      ref.onDispose(() {
        _taskHistoryToken?.cancel();
        _messagesHistoryToken?.cancel();
        unawaited(_wsSubscription?.cancel());
        _wsSubscription = null;
      });
      final ws = ref.read(webSocketServiceProvider);
      _wsSubscription = ws.events.listen(_onWsClientEvent);
      try {
        ws.connect(projectId);
      } on StateError catch (e, st) {
        log(
          'WS connect failed at TaskDetailController.build',
          name: 'TaskDetailController',
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

    return TaskDetailState.initial();
  }

  bool _realtimeBlocksMutations() {
    final v = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    return v?.realtimeMutationBlocked == true;
  }

  TaskMutationOutcome? _mutationSurfaceGuard() {
    if (state is! AsyncData<TaskDetailState>) {
      return TaskMutationOutcome.notReady;
    }
    return null;
  }

  /// [AsyncError] с тем же [error]/[stackTrace], value-слой из [data] (Riverpod
  /// [AbstractAsyncValueX.copyWithPrevious]).
  AsyncValue<TaskDetailState> _asyncErrorWithPreservedData(
    Object error,
    StackTrace stackTrace,
    AsyncData<TaskDetailState> data,
  ) {
    // ignore: invalid_use_of_internal_member
    final merged = AsyncError<TaskDetailState>(error, stackTrace).copyWithPrevious(data);
    return merged;
  }

  /// Патчит [TaskDetailState] и при [AsyncError] с [copyWithPrevious] сохраняет
  /// тот же error/stack — иначе WS/REST-апдейты после частичного фейла глотаются.
  void _patchState(TaskDetailState Function(TaskDetailState s) fn) {
    final cur = state;
    final v = switch (cur) {
      AsyncData<TaskDetailState>(:final value) => value,
      AsyncError<TaskDetailState>() when cur.hasValue => cur.requireValue,
      AsyncLoading<TaskDetailState>() when cur.hasValue => cur.requireValue,
      _ => null,
    };
    if (v == null) {
      return;
    }
    final next = fn(v);
    switch (cur) {
      case AsyncData<TaskDetailState>():
        state = AsyncData<TaskDetailState>(next);
      case AsyncError<TaskDetailState>(:final error, :final stackTrace)
          when cur.hasValue:
        state = _asyncErrorWithPreservedData(
          error,
          stackTrace,
          AsyncData<TaskDetailState>(next),
        );
      case AsyncLoading<TaskDetailState>() when cur.hasValue:
        // Редко: invalidate/reload с сохранённым value; иначе WS-патч пропадёт.
        // ignore: invalid_use_of_internal_member
        state = const AsyncLoading<TaskDetailState>().copyWithPrevious(
          AsyncData<TaskDetailState>(next),
        );
      default:
        break;
    }
  }

  /// UX: при ошибке POST после успешной загрузки карточки не затираем ленту/шапку ([copyWithPrevious]).
  void _setAsyncErrorPreservingPrevious(Object e, StackTrace st) {
    final prev = state;
    if (prev is AsyncData<TaskDetailState>) {
      state = _asyncErrorWithPreservedData(e, st, prev);
    } else {
      state = AsyncError<TaskDetailState>(e, st);
    }
  }

  void _scheduleWsRefetch(Future<void> Function() job) {
    if (_wsRefetchInFlight) {
      return;
    }
    _wsRefetchInFlight = true;
    unawaited(job().whenComplete(() => _wsRefetchInFlight = false));
  }

  /// Реконсиляция карточки после WS FSM mismatch; [TaskNotFoundException] — задача уже удалена на бэкенде.
  void _scheduleWsReconcileFromServer() {
    _scheduleWsRefetch(() async {
      try {
        await _reloadTaskReconcileFromServer();
      } on TaskNotFoundException {
        ref.invalidate(taskListControllerProvider(projectId: _projectId));
        _patchState(
          (s) => s.copyWith(
            task: null,
            taskDeleted: true,
            messages: const [],
            hasMoreMessages: false,
          ),
        );
      } catch (e, st) {
        log(
          'WS reconcile getTask failed (UI unchanged; consider snack/state field in 12.5)',
          name: 'TaskDetailController',
          error: e,
          stackTrace: st,
        );
      }
    });
  }

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
    if (!state.hasValue) {
      return;
    }
    final v = state.requireValue;
    if (v.realtimeServiceFailure == null) {
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
        // Как [ChatController]: server-кадр сбрасывает transient до фильтра projectId
        // в payload (возможен ложный сброс при событии другого проекта — как в Chat).
        _clearRealtimeTransientFailure();
        event.when(
          taskStatus: applyWsTaskStatus,
          taskMessage: applyWsTaskMessage,
          agentLog: (_) {},
          error: (err) {
            if (err.projectId != _projectId) {
              return;
            }
            if (err.needsRestRefetch) {
              requestRestRefetch();
            }
          },
          integrationStatus: (_) {},
          unknown: (_) {},
        );
        return;
    }
  }

  void applyRealtimeFailure(WsServiceFailure failure) {
    final mapped = mapWsServiceFailureForTasks(failure);
    switch (mapped.kind) {
      case TaskRealtimeFailureKind.transient:
        _patchState((s) => s.copyWith(realtimeServiceFailure: failure));
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
    _taskEpoch++;
    _messagesEpoch++;
    _lifecycleEpoch++;
    _taskHistoryToken?.cancel();
    _messagesHistoryToken?.cancel();
    _taskHistoryToken = CancelToken();
    _messagesHistoryToken = CancelToken();

    final prev = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };

    state = AsyncData(
      TaskDetailState(
        task: prev?.task,
        messages: const [],
        messagesTotal: 0,
        messagesOffset: 0,
        hasMoreMessages: false,
        messageTypeFilter: prev?.messageTypeFilter,
        senderTypeFilter: prev?.senderTypeFilter,
        isLoadingTask: true,
        isLoadingMessages: true,
        taskDeleted: false,
        realtimeMutationBlocked: prev?.realtimeMutationBlocked ?? false,
        realtimeSessionFailure: prev?.realtimeSessionFailure,
        realtimeServiceFailure: prev?.realtimeServiceFailure,
        messagesLoadMoreError: null,
        lifecycleMutationInFlight: null,
      ),
    );

    await _loadInitial(clearRealtimeBlocksOnSuccess: clearRealtimeBlocksOnSuccess);
  }

  Future<void> _loadInitial({bool clearRealtimeBlocksOnSuccess = false}) async {
    final te = _taskEpoch;
    final me = _messagesEpoch;
    final tTok = _taskHistoryToken;
    final mTok = _messagesHistoryToken;
    if (tTok == null || mTok == null) {
      return;
    }

    final repo = ref.read(taskRepositoryProvider);

    try {
      final task = await repo.getTask(_taskId, cancelToken: tTok);
      if (te != _taskEpoch) {
        return;
      }
      if (task.projectId != _projectId) {
        throw TaskDetailProjectMismatchException(
          taskId: task.id,
          expectedProjectId: _projectId,
          actualProjectId: task.projectId,
        );
      }

      _patchState(
        (s) => s.copyWith(
          task: _pickFresherTask(s.task, task),
          isLoadingTask: false,
        ),
      );

      final msgFilterType = switch (state) {
        AsyncData<TaskDetailState>(:final value) => value.messageTypeFilter,
        _ => null,
      };
      final msgFilterSender = switch (state) {
        AsyncData<TaskDetailState>(:final value) => value.senderTypeFilter,
        _ => null,
      };

      final page = await repo.listTaskMessages(
        _taskId,
        messageType: msgFilterType,
        senderType: msgFilterSender,
        limit: kTaskListDefaultLimit,
        offset: 0,
        cancelToken: mTok,
      );
      if (me != _messagesEpoch) {
        return;
      }

      final cur = switch (state) {
        AsyncData<TaskDetailState>(:final value) => value,
        _ => null,
      };
      if (cur == null) {
        return;
      }

      final idsBefore = cur.messages.map((m) => m.id).toSet();
      final merged = mergeTaskMessagesCanonical(cur.messages, page.messages);
      final nextOff = page.messages.length;
      final hasMore = _hasMoreMessagePage(
        nextOffset: nextOff,
        total: page.total,
        idsBefore: idsBefore,
        page: page.messages,
      );

      _patchState(
        (s) => s.copyWith(
          messages: merged,
          messagesTotal: page.total,
          messagesOffset: nextOff,
          hasMoreMessages: hasMore,
          isLoadingMessages: false,
          messagesLoadMoreError: null,
          realtimeServiceFailure: null,
          realtimeSessionFailure:
              clearRealtimeBlocksOnSuccess ? null : s.realtimeSessionFailure,
          realtimeMutationBlocked:
              clearRealtimeBlocksOnSuccess ? false : s.realtimeMutationBlocked,
        ),
      );
    } on TaskCancelledException {
      if (te != _taskEpoch) {
        return;
      }
      _patchState(
        (s) => s.copyWith(isLoadingTask: false, isLoadingMessages: false),
      );
    } catch (e, st) {
      if (te != _taskEpoch) {
        return;
      }
      _patchState(
        (s) => s.copyWith(isLoadingTask: false, isLoadingMessages: false),
      );
      _setAsyncErrorPreservingPrevious(e, st);
    }
  }

  /// Фильтры ленты сообщений: **`null` = сбросить ось** в «без фильтра по этому полю».
  ///
  /// Чтобы изменить только один критерий, UI (12.5/12.8) должен взять текущие
  /// [TaskDetailState.messageTypeFilter] / [TaskDetailState.senderTypeFilter] из провайдера
  /// и передать их явно вместе с новым значением для второй оси.
  Future<void> setMessageFilters({
    String? messageType,
    String? senderType,
  }) async {
    _messagesEpoch++;
    _messagesHistoryToken?.cancel();
    _messagesHistoryToken = CancelToken();

    _patchState(
      (s) => s.copyWith(
        messageTypeFilter: messageType,
        senderTypeFilter: senderType,
        messages: const [],
        messagesOffset: 0,
        messagesTotal: 0,
        hasMoreMessages: false,
        isLoadingMessages: true,
        messagesLoadMoreError: null,
      ),
    );

    await _loadMessagesFirstPage();
  }

  Future<void> _loadMessagesFirstPage() async {
    final me = _messagesEpoch;
    final mTok = _messagesHistoryToken;
    if (mTok == null) {
      return;
    }

    final repo = ref.read(taskRepositoryProvider);
    final filters = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (filters == null) {
      return;
    }

    try {
      final page = await repo.listTaskMessages(
        _taskId,
        messageType: filters.messageTypeFilter,
        senderType: filters.senderTypeFilter,
        limit: kTaskListDefaultLimit,
        offset: 0,
        cancelToken: mTok,
      );
      if (me != _messagesEpoch) {
        return;
      }

      final merged = mergeTaskMessagesCanonical(const [], page.messages);
      final nextOff = page.messages.length;
      final hasMore = _hasMoreMessagePage(
        nextOffset: nextOff,
        total: page.total,
        idsBefore: {},
        page: page.messages,
      );

      _patchState(
        (s) => s.copyWith(
          messages: merged,
          messagesTotal: page.total,
          messagesOffset: nextOff,
          hasMoreMessages: hasMore,
          isLoadingMessages: false,
          messagesLoadMoreError: null,
        ),
      );
    } on TaskCancelledException {
      if (me == _messagesEpoch) {
        _patchState((s) => s.copyWith(isLoadingMessages: false));
      }
    } catch (e) {
      if (me != _messagesEpoch) {
        return;
      }
      _patchState(
        (s) => s.copyWith(isLoadingMessages: false, messagesLoadMoreError: e),
      );
    }
  }

  /// Повтор после [TaskDetailState.messagesLoadMoreError] (догрузка или первая страница после фильтра).
  Future<void> retryMessagesAfterError() async {
    final cur = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (cur == null || cur.messagesLoadMoreError == null) {
      return;
    }

    final failedFirstPage =
        cur.messages.isEmpty && cur.messagesOffset == 0 && !cur.hasMoreMessages;
    if (failedFirstPage) {
      _patchState(
        (s) => s.copyWith(
          messagesLoadMoreError: null,
          isLoadingMessages: true,
        ),
      );
      await _loadMessagesFirstPage();
    } else {
      await loadMoreMessages();
    }
  }

  Future<void> loadMoreMessages() {
    final inflight = _loadMoreMessagesInFlight;
    if (inflight != null) {
      return inflight;
    }

    final s = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (s == null ||
        !s.hasMoreMessages ||
        s.isLoadingMessages ||
        s.isLoadingTask ||
        s.messagesOffset >= s.messagesTotal) {
      return Future.value();
    }

    final me = _messagesEpoch;
    final mTok = _messagesHistoryToken;
    if (mTok == null) {
      return Future.value();
    }

    late final Future<void> tracked;
    final f = _loadMoreMessagesImpl(me: me, token: mTok);
    tracked = f.whenComplete(() {
      if (identical(_loadMoreMessagesInFlight, tracked)) {
        _loadMoreMessagesInFlight = null;
      }
    });
    _loadMoreMessagesInFlight = tracked;
    return tracked;
  }

  Future<void> _loadMoreMessagesImpl({
    required int me,
    required CancelToken token,
  }) async {
    final cur = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    if (cur == null) {
      return;
    }

    _patchState(
      (s) => s.copyWith(isLoadingMessages: true, messagesLoadMoreError: null),
    );

    final repo = ref.read(taskRepositoryProvider);
    try {
      final page = await repo.listTaskMessages(
        _taskId,
        messageType: cur.messageTypeFilter,
        senderType: cur.senderTypeFilter,
        limit: kTaskListDefaultLimit,
        offset: cur.messagesOffset,
        cancelToken: token,
      );
      if (me != _messagesEpoch) {
        return;
      }

      final after = switch (state) {
        AsyncData<TaskDetailState>(:final value) => value,
        _ => null,
      };
      if (after == null) {
        return;
      }

      final idsBefore = after.messages.map((m) => m.id).toSet();
      final merged = mergeTaskMessagesCanonical(after.messages, page.messages);
      final nextOff = after.messagesOffset + page.messages.length;
      final hasMore = _hasMoreMessagePage(
        nextOffset: nextOff,
        total: page.total,
        idsBefore: idsBefore,
        page: page.messages,
      );

      _patchState(
        (s) => s.copyWith(
          messages: merged,
          messagesTotal: page.total,
          messagesOffset: nextOff,
          hasMoreMessages: hasMore,
          isLoadingMessages: false,
          messagesLoadMoreError: null,
        ),
      );
    } on TaskCancelledException {
      if (me == _messagesEpoch) {
        _patchState((s) => s.copyWith(isLoadingMessages: false));
      }
    } catch (e) {
      if (me != _messagesEpoch) {
        return;
      }
      _patchState(
        (s) => s.copyWith(isLoadingMessages: false, messagesLoadMoreError: e),
      );
    }
  }

  Future<void> _reloadTaskOnly() async {
    final te = _taskEpoch;
    final repo = ref.read(taskRepositoryProvider);
    final task = await repo.getTask(_taskId, cancelToken: null);
    if (te != _taskEpoch) {
      return;
    }
    if (task.projectId != _projectId) {
      throw TaskDetailProjectMismatchException(
        taskId: task.id,
        expectedProjectId: _projectId,
        actualProjectId: task.projectId,
      );
    }
    _patchState((s) => s.copyWith(task: _pickFresherTask(s.task, task)));
  }

  /// Явная реконсиляция при FSM mismatch по WS: доверяем снимку `getTask`, без слияния по `updatedAt`.
  Future<void> _reloadTaskReconcileFromServer() async {
    final te = _taskEpoch;
    final repo = ref.read(taskRepositoryProvider);
    final task = await repo.getTask(_taskId, cancelToken: null);
    if (te != _taskEpoch) {
      return;
    }
    if (task.projectId != _projectId) {
      throw TaskDetailProjectMismatchException(
        taskId: task.id,
        expectedProjectId: _projectId,
        actualProjectId: task.projectId,
      );
    }
    _patchState((s) => s.copyWith(task: task));
  }

  Future<TaskMutationOutcome> _runLifecycleMutation(
    TaskLifecycleMutation kind,
    Future<TaskModel> Function(TaskRepository repo) run,
  ) async {
    final guard = _mutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    _patchState((s) => s.copyWith(lifecycleMutationInFlight: kind));
    final epoch = ++_lifecycleEpoch;
    try {
      final repo = ref.read(taskRepositoryProvider);
      final t = await run(repo);
      if (!ref.mounted || epoch != _lifecycleEpoch) {
        log(
          'lifecycle mutation late success dropped (epoch changed or disposed)',
          name: 'TaskDetailController',
        );
        return TaskMutationOutcome.completed;
      }
      _patchState((s) => s.copyWith(task: _pickFresherTask(s.task, t)));
      ref
          .read(taskListControllerProvider(projectId: _projectId).notifier)
          .syncListFromHttpTask(t);
      return TaskMutationOutcome.completed;
    } catch (e, st) {
      if (ref.mounted && epoch == _lifecycleEpoch) {
        // Race-кейс: backend вернул 409 task_already_terminal — задача уже завершена
        // (worker успел финализировать между чтением state на UI и POST /cancel).
        // НЕ выставляем AsyncError (красный snack пугает на штатной ситуации);
        // перечитываем task из БД и возвращаем alreadyTerminal — UI покажет info-toast.
        if (e is TaskAlreadyTerminalException) {
          unawaited(_reloadTaskReconcileFromServer().catchError((_) {}));
          return TaskMutationOutcome.alreadyTerminal;
        }
        _setAsyncErrorPreservingPrevious(e, st);
        rethrow;
      }
      log(
        'lifecycle mutation late error dropped (epoch changed or disposed)',
        name: 'TaskDetailController',
        error: e,
        stackTrace: st,
      );
      return TaskMutationOutcome.completed;
    } finally {
      if (ref.mounted && epoch == _lifecycleEpoch) {
        _patchState((s) => s.copyWith(lifecycleMutationInFlight: null));
      }
    }
  }

  Future<TaskMutationOutcome> cancelTask() => _runLifecycleMutation(
        TaskLifecycleMutation.cancel,
        (repo) => repo.cancelTask(_taskId, cancelToken: null),
      );

  /// Sprint 17 / 6.10 — Pause v2: state='active' → state='paused'.
  Future<TaskMutationOutcome> pauseTask() => _runLifecycleMutation(
        TaskLifecycleMutation.pause,
        (repo) => repo.pauseTask(_taskId, cancelToken: null),
      );

  /// Sprint 17 / 6.10 — Resume v2: state='paused'|'needs_human'|'failed' → 'active'.
  Future<TaskMutationOutcome> resumeTask() => _runLifecycleMutation(
        TaskLifecycleMutation.resume,
        (repo) => repo.resumeTask(_taskId, cancelToken: null),
      );

  Future<TaskMutationOutcome> correctTask(String text) async {
    if (text.trim().isEmpty) {
      return TaskMutationOutcome.validationFailed;
    }
    if (utf8.encode(text).length > kUserCorrectionMaxBytes) {
      return TaskMutationOutcome.validationFailed;
    }
    final guard = _mutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    try {
      final repo = ref.read(taskRepositoryProvider);
      final t = await repo.correctTask(
        _taskId,
        CorrectTaskRequest(text: text),
        cancelToken: null,
      );
      _patchState((s) => s.copyWith(task: _pickFresherTask(s.task, t)));
      ref
          .read(taskListControllerProvider(projectId: _projectId).notifier)
          .syncListFromHttpTask(t);
      return TaskMutationOutcome.completed;
    } catch (e, st) {
      _setAsyncErrorPreservingPrevious(e, st);
      rethrow;
    }
  }

  Future<TaskMutationOutcome> updateTask(UpdateTaskRequest request) async {
    if (request.assignedAgentId != null && request.clearAssignedAgent) {
      return TaskMutationOutcome.validationFailed;
    }
    final guard = _mutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    try {
      final repo = ref.read(taskRepositoryProvider);
      final t = await repo.updateTask(_taskId, request, cancelToken: null);
      _patchState((s) => s.copyWith(task: _pickFresherTask(s.task, t)));
      ref
          .read(taskListControllerProvider(projectId: _projectId).notifier)
          .syncListFromHttpTask(t);
      return TaskMutationOutcome.completed;
    } catch (e, st) {
      _setAsyncErrorPreservingPrevious(e, st);
      rethrow;
    }
  }

  Future<TaskMutationOutcome> deleteTask() async {
    final guard = _mutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    try {
      final repo = ref.read(taskRepositoryProvider);
      await repo.deleteTask(_taskId, cancelToken: null);
      ref.invalidate(taskListControllerProvider(projectId: _projectId));
      _patchState(
        (s) => s.copyWith(
          task: null,
          taskDeleted: true,
          messages: const [],
          hasMoreMessages: false,
        ),
      );
      return TaskMutationOutcome.completed;
    } catch (e, st) {
      _setAsyncErrorPreservingPrevious(e, st);
      rethrow;
    }
  }

  /// Идемпотентность не гарантируется репозиторием — см. [AppLocalizations.taskSendMessageNoIdempotencyHint].
  Future<TaskMutationOutcome> sendTaskMessage(
    CreateTaskMessageRequest request,
  ) async {
    final guard = _mutationSurfaceGuard();
    if (guard != null) {
      return guard;
    }
    if (_realtimeBlocksMutations()) {
      return TaskMutationOutcome.blockedByRealtime;
    }
    try {
      final repo = ref.read(taskRepositoryProvider);
      final msg =
          await repo.addTaskMessage(_taskId, request, cancelToken: null);
      _patchState(
        (s) => s.copyWith(
          messages: mergeTaskMessagesCanonical(s.messages, [msg]),
        ),
      );
      return TaskMutationOutcome.completed;
    } catch (e, st) {
      _setAsyncErrorPreservingPrevious(e, st);
      rethrow;
    }
  }

  void applyWsTaskStatus(WsTaskStatusEvent e) {
    if (e.projectId != _projectId || e.taskId.isEmpty) {
      return;
    }

    final cur = switch (state) {
      AsyncData<TaskDetailState>(:final value) => value,
      _ => null,
    };
    final task = cur?.task;
    if (task == null) {
      return;
    }

    final own = <String>{
      _taskId,
      ...task.subTasks.map((x) => x.id),
    };
    if (!own.contains(e.taskId)) {
      return;
    }

    if (e.taskId == _taskId) {
      if (task.status != e.previousStatus) {
        _scheduleWsReconcileFromServer();
        return;
      }
      final next = task.copyWith(
        status: e.status,
        errorMessage: e.errorMessage,
      );
      _patchState((s) => s.copyWith(task: next));

      final aid = e.assignedAgentId;
      if (aid != null &&
          aid.isNotEmpty &&
          aid != task.assignedAgent?.id) {
        _scheduleWsRefetch(() async {
          try {
            await _reloadTaskOnly();
          } catch (e, st) {
            log(
              'WS agent-change reload getTask failed',
              name: 'TaskDetailController',
              error: e,
              stackTrace: st,
            );
          }
        });
      }
      return;
    }

    final subIdx = task.subTasks.indexWhere((st) => st.id == e.taskId);
    if (subIdx < 0) {
      return;
    }
    final sub = task.subTasks[subIdx];
    if (sub.status != e.previousStatus) {
      _scheduleWsReconcileFromServer();
      return;
    }
    final patchedSub = sub.copyWith(status: e.status);
    final nextSubs = List<TaskSummaryModel>.from(task.subTasks);
    nextSubs[subIdx] = patchedSub;
    _patchState((s) => s.copyWith(task: task.copyWith(subTasks: nextSubs)));
  }

  void applyWsTaskMessage(WsTaskMessageEvent e) {
    if (e.projectId != _projectId || e.taskId != _taskId) {
      return;
    }
    final m = wsTaskMessageToModel(e, taskId: _taskId);
    _patchState(
      (s) => s.copyWith(
        messages: mergeTaskMessagesCanonical(s.messages, [m]),
      ),
    );
  }
}
