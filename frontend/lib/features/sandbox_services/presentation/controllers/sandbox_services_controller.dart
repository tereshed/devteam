import 'package:dio/dio.dart';
import 'package:frontend/features/sandbox_services/data/sandbox_service_providers.dart';
import 'package:frontend/features/sandbox_services/domain/models/sandbox_service_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'sandbox_services_controller.g.dart';

/// Контроллер списка сервис-сайдкаров проекта: загрузка + upsert + delete.
@riverpod
class SandboxServicesController extends _$SandboxServicesController {
  @override
  Future<List<SandboxServiceModel>> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(sandboxServiceRepositoryProvider)
        .list(projectId, cancelToken: cancelToken);
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(
      () => ref.read(sandboxServiceRepositoryProvider).list(projectId),
    );
  }

  Future<void> upsert({
    required String alias,
    required bool isEnabled,
    String? kind,
    String? image,
    String? dbName,
    String? dbUser,
    int? port,
    String? seedKind,
    String? seedValue,
    int? readyTimeoutSeconds,
  }) async {
    await ref.read(sandboxServiceRepositoryProvider).upsert(
          projectId,
          alias: alias,
          isEnabled: isEnabled,
          kind: kind,
          image: image,
          dbName: dbName,
          dbUser: dbUser,
          port: port,
          seedKind: seedKind,
          seedValue: seedValue,
          readyTimeoutSeconds: readyTimeoutSeconds,
        );
    await refresh();
  }

  Future<void> delete(String serviceId) async {
    await ref
        .read(sandboxServiceRepositoryProvider)
        .delete(projectId, serviceId);
    await refresh();
  }
}
