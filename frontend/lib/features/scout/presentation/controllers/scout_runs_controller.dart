import 'package:dio/dio.dart';
import 'package:frontend/features/scout/data/scout_providers.dart';
import 'package:frontend/features/scout/domain/models/scout_run_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'scout_runs_controller.g.dart';

/// Контроллер прогонов разведчика: список + ручной запуск.
@riverpod
class ScoutRunsController extends _$ScoutRunsController {
  @override
  Future<List<ScoutRunModel>> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(scoutRepositoryProvider)
        .listRuns(projectId, cancelToken: cancelToken);
  }

  /// Запускает разведку по постановке проблемы и перечитывает список.
  Future<ScoutRunModel> dispatch(String problem) async {
    final run =
        await ref.read(scoutRepositoryProvider).dispatch(projectId, problem: problem);
    await refresh();
    return run;
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(
      () => ref.read(scoutRepositoryProvider).listRuns(projectId),
    );
  }

  bool get hasRunningRun =>
      state.value?.any((r) => r.status == 'running') ?? false;
}
