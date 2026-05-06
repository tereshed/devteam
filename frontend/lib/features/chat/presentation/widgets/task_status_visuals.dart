import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Роль агента на карточке задачи (строка с бэкенда / WebSocket `agent_role`).
enum TaskCardAgentRole {
  worker,
  supervisor,
  orchestrator,
  planner,
  developer,
  reviewer,
  tester,
  devops,
}

/// Парсинг `agent_role` из события / метаданных.
TaskCardAgentRole? taskCardAgentRoleTryParse(String? raw) {
  if (raw == null || raw.isEmpty) {
    return null;
  }
  try {
    return TaskCardAgentRole.values.byName(raw);
  } on ArgumentError {
    return null;
  }
}

/// Локализованная подпись роли.
String taskCardAgentRoleLabel(AppLocalizations l10n, TaskCardAgentRole role) {
  return switch (role) {
    TaskCardAgentRole.worker => l10n.taskCardAgentRoleWorker,
    TaskCardAgentRole.supervisor => l10n.taskCardAgentRoleSupervisor,
    TaskCardAgentRole.orchestrator => l10n.taskCardAgentRoleOrchestrator,
    TaskCardAgentRole.planner => l10n.taskCardAgentRolePlanner,
    TaskCardAgentRole.developer => l10n.taskCardAgentRoleDeveloper,
    TaskCardAgentRole.reviewer => l10n.taskCardAgentRoleReviewer,
    TaskCardAgentRole.tester => l10n.taskCardAgentRoleTester,
    TaskCardAgentRole.devops => l10n.taskCardAgentRoleDevops,
  };
}

/// Известные статусы задачи (нормативный список ТЗ 11.7).
const kNormativeTaskStatuses = <String>[
  'pending',
  'planning',
  'in_progress',
  'review',
  'testing',
  'changes_requested',
  'completed',
  'failed',
  'cancelled',
  'paused',
];

/// `true`, если [status] входит в нормативный список.
bool taskStatusIsKnown(String status) => kNormativeTaskStatuses.contains(status);

/// Визуальная категория M3 для строки статуса задачи.
enum TaskStatusVisualCategory {
  active,
  success,
  error,
  stopped,

  /// Пустая строка, неизвестный литерал бэкенда — нейтральный бейдж (не «активный»).
  unknown,
}

TaskStatusVisualCategory taskStatusVisualCategory(String status) {
  return switch (status) {
    'pending' ||
    'planning' ||
    'in_progress' ||
    'review' ||
    'testing' ||
    'changes_requested' =>
      TaskStatusVisualCategory.active,
    'completed' => TaskStatusVisualCategory.success,
    'failed' => TaskStatusVisualCategory.error,
    'cancelled' || 'paused' => TaskStatusVisualCategory.stopped,
    _ => TaskStatusVisualCategory.unknown,
  };
}

Color taskStatusContainerColor(ColorScheme scheme, TaskStatusVisualCategory cat) {
  return switch (cat) {
    TaskStatusVisualCategory.active => scheme.secondaryContainer,
    TaskStatusVisualCategory.success => scheme.tertiaryContainer,
    TaskStatusVisualCategory.error => scheme.errorContainer,
    TaskStatusVisualCategory.stopped => scheme.surfaceContainerHighest,
    TaskStatusVisualCategory.unknown => scheme.surfaceContainerHighest,
  };
}

Color taskStatusOnContainerColor(ColorScheme scheme, TaskStatusVisualCategory cat) {
  return switch (cat) {
    TaskStatusVisualCategory.active => scheme.onSecondaryContainer,
    TaskStatusVisualCategory.success => scheme.onTertiaryContainer,
    TaskStatusVisualCategory.error => scheme.onErrorContainer,
    TaskStatusVisualCategory.stopped => scheme.onSurfaceVariant,
    TaskStatusVisualCategory.unknown => scheme.onSurfaceVariant,
  };
}

IconData taskStatusIcon(TaskStatusVisualCategory cat) {
  return switch (cat) {
    TaskStatusVisualCategory.active => Icons.autorenew,
    TaskStatusVisualCategory.success => Icons.check_circle,
    TaskStatusVisualCategory.error => Icons.error,
    TaskStatusVisualCategory.stopped => Icons.pause_circle,
    TaskStatusVisualCategory.unknown => Icons.pause_circle,
  };
}

/// Локализованная подпись статуса; неизвестная строка → [AppLocalizations.taskStatusUnknownStatus].
///
/// Ветка `_` покрывает пустую строку, неизвестные литералы и рассинхрон с [kNormativeTaskStatuses]
/// (новый статус в списке без добавления ветки здесь).
String taskStatusLabel(AppLocalizations l10n, String status) {
  return switch (status) {
    'pending' => l10n.taskStatusPending,
    'planning' => l10n.taskStatusPlanning,
    'in_progress' => l10n.taskStatusInProgress,
    'review' => l10n.taskStatusReview,
    'testing' => l10n.taskStatusTesting,
    'changes_requested' => l10n.taskStatusChangesRequested,
    'completed' => l10n.taskStatusCompleted,
    'failed' => l10n.taskStatusFailed,
    'cancelled' => l10n.taskStatusCancelled,
    'paused' => l10n.taskStatusPaused,
    _ => l10n.taskStatusUnknownStatus,
  };
}
