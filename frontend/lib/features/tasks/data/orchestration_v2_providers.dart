import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_repository.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';

final orchestrationV2RepositoryProvider =
    Provider<OrchestrationV2Repository>((ref) {
  return OrchestrationV2Repository(dio: ref.watch(dioClientProvider));
});

final taskArtifactsProvider =
    FutureProvider.autoDispose.family<List<Artifact>, String>((ref, taskId) {
  return ref.watch(orchestrationV2RepositoryProvider).listArtifacts(taskId);
});

/// Identifier для запроса полного артефакта: (taskId, artifactId).
///
/// Reverse-name: tuple фиксирован, Equatable не нужен — record-types в Dart 3
/// получают value-equality бесплатно.
typedef ArtifactDetailId = (String taskId, String artifactId);

final artifactDetailProvider = FutureProvider.autoDispose
    .family<Artifact, ArtifactDetailId>((ref, id) {
  return ref
      .watch(orchestrationV2RepositoryProvider)
      .getArtifact(id.$1, id.$2);
});

final taskRouterDecisionsProvider = FutureProvider.autoDispose
    .family<List<RouterDecision>, String>((ref, taskId) {
  return ref
      .watch(orchestrationV2RepositoryProvider)
      .listRouterDecisions(taskId);
});
