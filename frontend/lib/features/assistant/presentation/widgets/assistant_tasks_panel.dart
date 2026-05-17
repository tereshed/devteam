import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/domain/assistant_active_task_model.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_tasks_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Канонические V2-состояния задачи (см. `backend/internal/models/task.go`
/// — `TaskState`, Sprint 17 / Orchestration v2). Legacy V1-статусы
/// (planning/in_progress/review/testing/completed) **удалены** в Sprint 5.
///
/// Используем как enum-подобный switch источник цвета/локализации, чтобы
/// будущие новые состояния (если появятся) ловились как `default` без
/// «silent fallback» на legacy.
const String kAssistantTaskStateActive = 'active';
const String kAssistantTaskStateDone = 'done';
const String kAssistantTaskStateFailed = 'failed';
const String kAssistantTaskStateCancelled = 'cancelled';
const String kAssistantTaskStateNeedsHuman = 'needs_human';
const String kAssistantTaskStatePaused = 'paused';

/// Вкладка «Tasks» правой панели (Sprint 21 §11 frontend).
///
/// Источники данных:
/// - REST bootstrap `GET /assistant/active-tasks` (на первой подписке);
/// - WS `assistant.task_update` (live апдейты).
class AssistantTasksPanel extends ConsumerStatefulWidget {
  const AssistantTasksPanel({super.key});

  @override
  ConsumerState<AssistantTasksPanel> createState() =>
      _AssistantTasksPanelState();
}

class _AssistantTasksPanelState extends ConsumerState<AssistantTasksPanel> {
  bool _bootstrapped = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || _bootstrapped) {
        return;
      }
      _bootstrapped = true;
      // Ошибки уже сохраняются внутри controller.refresh() в state.error.
      // catchError + unawaited — защита от unhandled future в тестах.
      unawaited(
        ref
            .read(assistantTasksControllerProvider.notifier)
            .refresh()
            .catchError((Object _) {}),
      );
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AssistantTasksPanel');
    final state = ref.watch(assistantTasksControllerProvider);

    if (state.loading && state.tasks.isEmpty) {
      return const Center(child: CircularProgressIndicator(strokeWidth: 2));
    }

    if (state.error != null && state.tasks.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                l10n.assistantErrorGeneric,
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              const SizedBox(height: 8),
              FilledButton.icon(
                onPressed: () => ref
                    .read(assistantTasksControllerProvider.notifier)
                    .refresh(),
                icon: const Icon(Icons.refresh),
                label: Text(l10n.assistantRetry),
              ),
            ],
          ),
        ),
      );
    }

    if (state.tasks.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(
            l10n.assistantNoActiveTasks,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => ref
          .read(assistantTasksControllerProvider.notifier)
          .refresh(),
      child: ListView.builder(
        padding: const EdgeInsets.symmetric(vertical: 4),
        itemCount: state.tasks.length,
        itemBuilder: (context, index) {
          final t = state.tasks[index];
          return _ActiveTaskCard(task: t);
        },
      ),
    );
  }
}

class _ActiveTaskCard extends StatelessWidget {
  const _ActiveTaskCard({required this.task});

  final AssistantActiveTaskModel task;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_ActiveTaskCard');
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      child: InkWell(
        onTap: () {
          // Маршрут project_dashboard_routes.dart: /projects/:id/tasks/:taskId.
          // (Если в реальном роутере путь иной — поправится централизованно.)
          context.go('/projects/${task.projectId}/tasks/${task.taskId}');
        },
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      task.title.isEmpty ? '—' : task.title,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  _StateChip(state: task.state),
                ],
              ),
              const SizedBox(height: 4),
              Text(
                task.projectName.isEmpty ? '—' : task.projectName,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 4),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    task.updatedAt.toLocal().toIso8601String(),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: scheme.onSurfaceVariant,
                    ),
                  ),
                  TextButton.icon(
                    onPressed: () => context.go(
                        '/projects/${task.projectId}/tasks/${task.taskId}'),
                    icon: const Icon(Icons.arrow_forward, size: 14),
                    label: Text(l10n.assistantOpenTask),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _StateChip extends StatelessWidget {
  const _StateChip({required this.state});

  final String state;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_StateChip');
    final scheme = Theme.of(context).colorScheme;
    final (bg, fg) = switch (state) {
      kAssistantTaskStateActive =>
        (scheme.secondaryContainer, scheme.onSecondaryContainer),
      kAssistantTaskStateDone =>
        (scheme.tertiaryContainer, scheme.onTertiaryContainer),
      kAssistantTaskStateFailed || kAssistantTaskStateCancelled =>
        (scheme.errorContainer, scheme.onErrorContainer),
      kAssistantTaskStateNeedsHuman || kAssistantTaskStatePaused =>
        (scheme.surfaceContainerHighest, scheme.onSurfaceVariant),
      // Будущие/неизвестные стейты — нейтральный нейтрал. Не молчим про
      // «completed», «in_progress» и т.п.: они V1 и в БД невозможны после
      // Sprint 5 — увидев такое, мы знаем, что бэкенд сломался.
      _ => (scheme.surfaceContainerHighest, scheme.onSurfaceVariant),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration:
          BoxDecoration(color: bg, borderRadius: BorderRadius.circular(8)),
      child: Text(
        assistantTaskStateLabel(l10n, state),
        style:
            Theme.of(context).textTheme.labelSmall?.copyWith(color: fg),
      ),
    );
  }
}

/// Локализованная подпись V2-состояния задачи. Неизвестная строка —
/// fallback на сырое значение (видно отладчику, не падаем).
String assistantTaskStateLabel(AppLocalizations l10n, String state) {
  switch (state) {
    case kAssistantTaskStateActive:
      return l10n.assistantTaskStateActive;
    case kAssistantTaskStateDone:
      return l10n.assistantTaskStateDone;
    case kAssistantTaskStateFailed:
      return l10n.assistantTaskStateFailed;
    case kAssistantTaskStateCancelled:
      return l10n.assistantTaskStateCancelled;
    case kAssistantTaskStateNeedsHuman:
      return l10n.assistantTaskStateNeedsHuman;
    case kAssistantTaskStatePaused:
      return l10n.assistantTaskStatePaused;
    default:
      return state;
  }
}
