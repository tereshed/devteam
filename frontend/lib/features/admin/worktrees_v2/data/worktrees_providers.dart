import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/worktrees_v2/data/worktrees_repository.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';

final worktreesRepositoryProvider = Provider<WorktreesRepository>((ref) {
  final dio = ref.watch(dioClientProvider);
  return WorktreesRepository(dio: dio);
});

/// Хранит текущий фильтр по state для глобального debug-экрана worktrees.
/// `null` означает «без фильтра» (все состояния).
///
/// Использован простой Notifier вместо @riverpod-кодогена, чтобы не плодить
/// .g.dart-файлы для одного-единственного nullable-стейта.
class WorktreesStateFilter extends Notifier<String?> {
  @override
  String? build() => null;

  void set(String? value) => state = value;
}

final worktreesStateFilterProvider =
    NotifierProvider<WorktreesStateFilter, String?>(WorktreesStateFilter.new);

/// Список worktrees с учётом текущего фильтра.
final worktreesListProvider =
    FutureProvider.autoDispose<List<WorktreeV2>>((ref) async {
  final repo = ref.watch(worktreesRepositoryProvider);
  final state = ref.watch(worktreesStateFilterProvider);
  return repo.list(state: state);
});
