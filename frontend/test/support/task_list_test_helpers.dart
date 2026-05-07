import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';

/// Ждёт завершения первичной загрузки [taskListControllerProvider] (или AsyncError).
///
/// Вынесено из тестов списка/деталей задач (12.3); переиспользовать в следующих 12.x.
Future<void> waitTaskListControllerIdle(
  ProviderContainer container,
  String projectId, {
  Duration step = const Duration(milliseconds: 4),
  Duration timeout = const Duration(seconds: 3),
}) async {
  final sw = Stopwatch()..start();
  while (sw.elapsed < timeout) {
    final st = container.read(taskListControllerProvider(projectId: projectId));
    if (st.hasError) {
      return;
    }
    if (st.hasValue && !st.requireValue.isLoadingInitial) {
      return;
    }
    await Future<void>.delayed(step);
  }
  fail('timeout waitTaskListControllerIdle($projectId)');
}
