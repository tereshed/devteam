import 'package:dio/dio.dart';
import 'package:frontend/features/enhancer/data/enhancer_providers.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_config_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'enhancer_config_controller.g.dart';

/// Контроллер конфига энхансера проекта: загрузка + сохранение.
@riverpod
class EnhancerConfigController extends _$EnhancerConfigController {
  @override
  Future<EnhancerConfigModel> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(enhancerRepositoryProvider)
        .getConfig(projectId, cancelToken: cancelToken);
  }

  Future<void> save({
    bool? isActive,
    String? cronExpression,
    int? analysisWindowDays,
    int? maxChangesPerRun,
  }) async {
    final updated = await ref.read(enhancerRepositoryProvider).updateConfig(
          projectId,
          isActive: isActive,
          cronExpression: cronExpression,
          analysisWindowDays: analysisWindowDays,
          maxChangesPerRun: maxChangesPerRun,
        );
    state = AsyncValue.data(updated);
  }
}
