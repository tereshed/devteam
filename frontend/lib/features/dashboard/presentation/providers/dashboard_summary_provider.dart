import 'package:dio/dio.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/requests.dart' as projects;
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'dashboard_summary_provider.g.dart';

/// Сводка для hub-экрана `/dashboard`.
///
/// Этап 1 ui_refactoring: подключение LLM/Git только в этапах 2/3 — поэтому
/// эти счётчики обнулены до соответствующих этапов.
class DashboardSummary {
  final int projectsActive;
  final int projectsTotal;
  final int agentsTotal;
  final int llmConnected;
  final int gitConnected;

  const DashboardSummary({
    required this.projectsActive,
    required this.projectsTotal,
    required this.agentsTotal,
    required this.llmConnected,
    required this.gitConnected,
  });
}

/// Считает счётчики проектов/агентов параллельно. LLM/Git — заглушки до этапов 2/3.
@riverpod
Future<DashboardSummary> dashboardSummary(Ref ref) async {
  final projectsFuture = ref.watch(
    projectListProvider(limit: 1, offset: 0).future,
  );
  final agentsFuture = ref.watch(agentsV2ListProvider.future);

  final results = await Future.wait([projectsFuture, agentsFuture]);
  final projectsResp = results[0] as projects.ProjectListResponse;
  final agents = results[1] as AgentV2Page;

  // Активные проекты — отдельный запрос с фильтром для точного числа;
  // total из пагинации основной выборки даёт общий total.
  // ref.watch (а не read) — иначе провайдер не пересчитается при инвалидации
  // projectListProvider (создание/удаление проекта в другом месте UI).
  final activeResp = await ref.watch(
    projectListProvider(
      limit: 1,
      offset: 0,
      filter: const projects.ProjectListFilter(status: 'active'),
    ).future,
  );

  return DashboardSummary(
    projectsTotal: projectsResp.total,
    projectsActive: activeResp.total,
    agentsTotal: agents.total,
    llmConnected: 0,
    gitConnected: 0,
  );
}

/// Последние задачи (до 5) из первого доступного проекта пользователя.
///
/// На бэке нет глобального `/me/tasks`-эндпоинта; на этапе 1 это
/// «недорогой» first-fit: берём первый проект и его задачи. Когда появится
/// глобальный feed (бэкенд-таска вне scope этапа 1) — провайдер заменим.
@riverpod
Future<List<TaskListItemModel>> dashboardRecentTasks(Ref ref) async {
  final projectsResp = await ref.watch(
    projectListProvider(limit: 1, offset: 0).future,
  );
  if (projectsResp.projects.isEmpty) {
    return const [];
  }

  final firstProject = projectsResp.projects.first;
  final cancelToken = CancelToken();
  ref.onDispose(cancelToken.cancel);

  final repo = ref.watch(taskRepositoryProvider);
  final response = await repo.listTasks(
    firstProject.id,
    limit: 5,
    offset: 0,
    cancelToken: cancelToken,
  );
  return response.tasks;
}
