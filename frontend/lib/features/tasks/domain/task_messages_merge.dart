import 'package:frontend/features/tasks/domain/models.dart';

/// Слияние сообщений задачи по [TaskMessageModel.id], сортировка `created_at` ASC, tie-breaker `id` ASC.
List<TaskMessageModel> mergeTaskMessagesCanonical(
  List<TaskMessageModel> current,
  List<TaskMessageModel> incoming,
) {
  final byId = <String, TaskMessageModel>{};
  for (final m in current) {
    byId[m.id] = m;
  }
  for (final m in incoming) {
    byId[m.id] = m;
  }
  final out = byId.values.toList()
    ..sort((a, b) {
      final c = a.createdAt.compareTo(b.createdAt);
      if (c != 0) {
        return c;
      }
      return a.id.compareTo(b.id);
    });
  return out;
}
