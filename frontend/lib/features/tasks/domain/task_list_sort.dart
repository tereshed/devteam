import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';

/// Сортировка строк списка по полям [TaskListFilter.orderBy] / [TaskListFilter.orderDir].
///
/// При равенстве основного ключа порядок фиксируется по `id` ascending (стабильный
/// tie-break; для `created_at` DESC это даёт лексикографически возрастающие id внутри
/// одной даты).
void sortTaskListItems(List<TaskListItemModel> items, TaskListFilter filter) {
  items.sort((a, b) => compareTaskListItems(a, b, filter));
}

int compareTaskListItems(
  TaskListItemModel a,
  TaskListItemModel b,
  TaskListFilter filter,
) {
  final dir = filter.orderDir ?? TaskOrderDir.desc;
  final asc = dir == TaskOrderDir.asc ? 1 : -1;
  final ob = filter.orderBy?.trim().toLowerCase();
  int cmp;
  switch (ob) {
    case 'updated_at':
      cmp = a.updatedAt.compareTo(b.updatedAt);
      break;
    case 'title':
      cmp = a.title.compareTo(b.title);
      break;
    case 'priority':
      cmp = a.priority.compareTo(b.priority);
      break;
    case 'status':
      cmp = a.status.compareTo(b.status);
      break;
    case 'created_at':
    default:
      cmp = a.createdAt.compareTo(b.createdAt);
      break;
  }
  if (cmp != 0) {
    return asc * cmp;
  }
  return a.id.compareTo(b.id);
}
