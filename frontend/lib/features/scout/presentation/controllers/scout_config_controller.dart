import 'package:dio/dio.dart';
import 'package:frontend/features/scout/data/scout_providers.dart';
import 'package:frontend/features/scout/domain/models/scout_config_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'scout_config_controller.g.dart';

/// Контроллер конфига разведчика проекта: загрузка + сохранение.
@riverpod
class ScoutConfigController extends _$ScoutConfigController {
  @override
  Future<ScoutConfigModel> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(scoutRepositoryProvider)
        .getConfig(projectId, cancelToken: cancelToken);
  }

  Future<void> save({
    bool? isEnabled,
    String? prompt,
    String? codeBackend,
    String? providerKind,
    double? temperature,
    Map<String, dynamic>? codeBackendSettings,
    Map<String, dynamic>? sandboxPermissions,
    String? subscriptionId,
    int? timeoutSeconds,
  }) async {
    final updated = await ref.read(scoutRepositoryProvider).updateConfig(
          projectId,
          isEnabled: isEnabled,
          prompt: prompt,
          codeBackend: codeBackend,
          providerKind: providerKind,
          temperature: temperature,
          codeBackendSettings: codeBackendSettings,
          sandboxPermissions: sandboxPermissions,
          subscriptionId: subscriptionId,
          timeoutSeconds: timeoutSeconds,
        );
    state = AsyncValue.data(updated);
  }
}
