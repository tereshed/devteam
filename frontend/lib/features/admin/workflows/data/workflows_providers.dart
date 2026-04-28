import 'package:frontend/features/admin/workflows/data/workflows_repository.dart';
import 'package:frontend/features/admin/workflows/domain/models.dart';
import 'package:frontend/features/auth/data/auth_providers.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'workflows_providers.g.dart';

@riverpod
WorkflowsRepository workflowsRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return WorkflowsRepository(dio);
}

@riverpod
Future<List<Workflow>> workflowsList(Ref ref) {
  return ref.watch(workflowsRepositoryProvider).getWorkflows();
}

@riverpod
Future<ExecutionListResponse> executionsList(
  Ref ref, {
  int limit = 20,
  int offset = 0,
}) {
  return ref.watch(workflowsRepositoryProvider).getExecutions(limit, offset);
}

@riverpod
Future<Execution> executionDetail(Ref ref, String id) {
  return ref.watch(workflowsRepositoryProvider).getExecution(id);
}

@riverpod
Future<List<ExecutionStep>> executionSteps(Ref ref, String id) {
  return ref.watch(workflowsRepositoryProvider).getExecutionSteps(id);
}
