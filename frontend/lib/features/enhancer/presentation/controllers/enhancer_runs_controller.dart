import 'package:dio/dio.dart';
import 'package:frontend/features/enhancer/data/enhancer_providers.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_change_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_run_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'enhancer_runs_controller.g.dart';

/// Контроллер прогонов энхансера: список + ручной запуск.
@riverpod
class EnhancerRunsController extends _$EnhancerRunsController {
  @override
  Future<List<EnhancerRunModel>> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(enhancerRepositoryProvider)
        .listRuns(projectId, cancelToken: cancelToken);
  }

  /// Запускает прогон и сразу перечитывает список (новый run появится сверху
  /// в состоянии running). 409 пробрасывается как EnhancerRunInProgressException.
  Future<EnhancerRunModel> runNow() async {
    final run = await ref.read(enhancerRepositoryProvider).runNow(projectId);
    await refresh();
    return run;
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(enhancerRepositoryProvider).listRuns(projectId),
    );
  }

  /// Есть ли незавершённый прогон (для авто-обновления списка в UI).
  bool get hasRunningRun =>
      state.value?.any((r) => r.status == 'running') ?? false;

  /// Применяет предложение и перечитывает список предложений прогона.
  Future<void> applyChange(String runId, String changeId) async {
    await ref.read(enhancerRepositoryProvider).applyChange(projectId, changeId);
    ref.invalidate(enhancerRunChangesProvider(projectId, runId));
  }

  /// Отклоняет предложение и перечитывает список.
  Future<void> rejectChange(String runId, String changeId) async {
    await ref
        .read(enhancerRepositoryProvider)
        .rejectChange(projectId, changeId);
    ref.invalidate(enhancerRunChangesProvider(projectId, runId));
  }

  /// Откатывает применённое предложение и перечитывает список.
  Future<void> rollbackChange(String runId, String changeId) async {
    await ref
        .read(enhancerRepositoryProvider)
        .rollbackChange(projectId, changeId);
    ref.invalidate(enhancerRunChangesProvider(projectId, runId));
  }
}

/// Предложения изменений одного прогона (грузятся лениво при раскрытии).
@riverpod
Future<List<EnhancerChangeModel>> enhancerRunChanges(
  Ref ref,
  String projectId,
  String runId,
) async {
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);
  return ref
      .read(enhancerRepositoryProvider)
      .listRunChanges(projectId, runId, cancelToken: cancelToken);
}
