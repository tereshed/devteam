import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/schedules/data/scheduled_task_repository.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'scheduled_task_providers.g.dart';

@Riverpod(keepAlive: true)
ScheduledTaskRepository scheduledTaskRepository(Ref ref) {
  final dio = ref.watch(dioClientProvider);
  return ScheduledTaskRepository(dio: dio);
}
