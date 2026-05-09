import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Тон отображения статуса задачи (Material 3 поверхности).
enum TaskStatusTone {
  active,
  success,
  error,
  stopped,
  unknown,
}

TaskStatusTone taskStatusTone(String status) {
  return switch (status) {
    'pending' ||
    'planning' ||
    'in_progress' ||
    'review' ||
    'testing' ||
    'changes_requested' =>
      TaskStatusTone.active,
    'completed' => TaskStatusTone.success,
    'failed' => TaskStatusTone.error,
    'cancelled' || 'paused' => TaskStatusTone.stopped,
    _ => TaskStatusTone.unknown,
  };
}

IconData taskStatusIcon(TaskStatusTone tone) {
  return switch (tone) {
    TaskStatusTone.active => Icons.autorenew,
    TaskStatusTone.success => Icons.check_circle_outline,
    TaskStatusTone.error => Icons.error_outline,
    TaskStatusTone.stopped => Icons.pause_circle_outline,
    TaskStatusTone.unknown => Icons.help_outline,
  };
}

Color taskStatusChipBackground(ColorScheme scheme, TaskStatusTone tone) {
  return switch (tone) {
    TaskStatusTone.active => scheme.secondaryContainer,
    TaskStatusTone.success => scheme.tertiaryContainer,
    TaskStatusTone.error => scheme.errorContainer,
    TaskStatusTone.stopped => scheme.surfaceContainerHighest,
    TaskStatusTone.unknown => scheme.surfaceContainerHighest,
  };
}

Color taskStatusChipForeground(ColorScheme scheme, TaskStatusTone tone) {
  return switch (tone) {
    TaskStatusTone.active => scheme.onSecondaryContainer,
    TaskStatusTone.success => scheme.onTertiaryContainer,
    TaskStatusTone.error => scheme.onErrorContainer,
    TaskStatusTone.stopped => scheme.onSurfaceVariant,
    TaskStatusTone.unknown => scheme.onSurfaceVariant,
  };
}

/// Локализованная подпись статуса ([taskStatusPending] … [taskStatusUnknownStatus]).
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

/// Тон приоритета задачи.
enum TaskPriorityTone {
  critical,
  high,
  medium,
  low,
  unknown,
}

TaskPriorityTone taskPriorityTone(String priority) {
  return switch (priority) {
    'critical' => TaskPriorityTone.critical,
    'high' => TaskPriorityTone.high,
    'medium' => TaskPriorityTone.medium,
    'low' => TaskPriorityTone.low,
    _ => TaskPriorityTone.unknown,
  };
}

IconData taskPriorityIcon(TaskPriorityTone tone) {
  return switch (tone) {
    TaskPriorityTone.critical => Icons.priority_high,
    TaskPriorityTone.high => Icons.keyboard_arrow_up,
    TaskPriorityTone.medium => Icons.drag_handle,
    TaskPriorityTone.low => Icons.keyboard_arrow_down,
    TaskPriorityTone.unknown => Icons.help_outline,
  };
}

Color taskPriorityChipBackground(ColorScheme scheme, TaskPriorityTone tone) {
  return switch (tone) {
    TaskPriorityTone.critical => scheme.errorContainer,
    TaskPriorityTone.high => scheme.primaryContainer,
    TaskPriorityTone.medium => scheme.secondaryContainer,
    TaskPriorityTone.low => scheme.surfaceContainerHighest,
    TaskPriorityTone.unknown => scheme.surfaceContainerHighest,
  };
}

Color taskPriorityChipForeground(ColorScheme scheme, TaskPriorityTone tone) {
  return switch (tone) {
    TaskPriorityTone.critical => scheme.onErrorContainer,
    TaskPriorityTone.high => scheme.onPrimaryContainer,
    TaskPriorityTone.medium => scheme.onSecondaryContainer,
    TaskPriorityTone.low => scheme.onSurfaceVariant,
    TaskPriorityTone.unknown => scheme.onSurfaceVariant,
  };
}

/// Локализованная подпись приоритета ([taskPriorityCritical] … [taskPriorityUnknown]).
String taskPriorityLabel(AppLocalizations l10n, String priority) {
  return switch (priority) {
    'critical' => l10n.taskPriorityCritical,
    'high' => l10n.taskPriorityHigh,
    'medium' => l10n.taskPriorityMedium,
    'low' => l10n.taskPriorityLow,
    _ => l10n.taskPriorityUnknown,
  };
}
