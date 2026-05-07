import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'task_providers.g.dart';

@Riverpod(keepAlive: true)
TaskRepository taskRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return TaskRepository(dio: dio);
}
