import 'package:dio/dio.dart';
import 'package:frontend/features/assistant_mcp/data/assistant_mcp_providers.dart';
import 'package:frontend/features/assistant_mcp/domain/models/assistant_mcp_server_model.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'assistant_mcp_controller.g.dart';

/// Контроллер списка MCP-серверов ассистента проекта: загрузка + create/update/delete.
@riverpod
class AssistantMcpController extends _$AssistantMcpController {
  @override
  Future<List<AssistantMcpServerModel>> build(String projectId) async {
    final cancelToken = CancelToken();
    ref.onDispose(cancelToken.cancel);
    return ref
        .read(assistantMcpRepositoryProvider)
        .list(projectId, cancelToken: cancelToken);
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(
      () => ref.read(assistantMcpRepositoryProvider).list(projectId),
    );
  }

  Future<void> create({
    required String name,
    required String transport,
    required String url,
    required Map<String, String> headers,
    required bool requireConfirmation,
    required bool isEnabled,
  }) async {
    await ref.read(assistantMcpRepositoryProvider).create(
          projectId,
          name: name,
          transport: transport,
          url: url,
          headers: headers,
          requireConfirmation: requireConfirmation,
          isEnabled: isEnabled,
        );
    await refresh();
  }

  Future<void> updateServer(
    String serverId, {
    required String name,
    required String transport,
    required String url,
    required Map<String, String> headers,
    required bool requireConfirmation,
    required bool isEnabled,
  }) async {
    await ref.read(assistantMcpRepositoryProvider).update(
          projectId,
          serverId,
          name: name,
          transport: transport,
          url: url,
          headers: headers,
          requireConfirmation: requireConfirmation,
          isEnabled: isEnabled,
        );
    await refresh();
  }

  /// Быстрое переключение enabled без открытия формы (полная замена полей).
  Future<void> toggleEnabled(
    AssistantMcpServerModel s,
    bool enabled,
  ) async {
    await updateServer(
      s.id,
      name: s.name,
      transport: s.transport,
      url: s.url,
      headers: s.headers,
      requireConfirmation: s.requireConfirmation,
      isEnabled: enabled,
    );
  }

  Future<void> delete(String serverId) async {
    await ref.read(assistantMcpRepositoryProvider).delete(projectId, serverId);
    await refresh();
  }
}
