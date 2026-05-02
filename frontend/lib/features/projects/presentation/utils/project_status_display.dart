import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

class ProjectStatusDisplay {
  final Color color;
  final IconData icon;
  final String label;

  const ProjectStatusDisplay({
    required this.color,
    required this.icon,
    required this.label,
  });
}

// TODO(design): Colors.orange и Colors.green не адаптируются к тёмной теме.
// При наличии дизайн-системы заменить на расширение ColorScheme
// (cs.warning / cs.success) или Color.fromARGB с учётом Brightness.
ProjectStatusDisplay projectStatusDisplay(BuildContext context, String status) {
  final l10n = AppLocalizations.of(context)!;
  final cs = Theme.of(context).colorScheme;

  return switch (status) {
    'active' => ProjectStatusDisplay(
        color: cs.primary,
        icon: Icons.play_circle_outline,
        label: l10n.statusActive,
      ),
    'paused' => ProjectStatusDisplay(
        color: Colors.orange,
        icon: Icons.pause_circle_outline,
        label: l10n.statusPaused,
      ),
    'archived' => ProjectStatusDisplay(
        color: cs.outline,
        icon: Icons.archive_outlined,
        label: l10n.statusArchived,
      ),
    'indexing' => ProjectStatusDisplay(
        color: cs.tertiary,
        icon: Icons.sync,
        label: l10n.statusIndexing,
      ),
    'indexing_failed' => ProjectStatusDisplay(
        color: cs.error,
        icon: Icons.error_outline,
        label: l10n.statusIndexingFailed,
      ),
    'ready' => ProjectStatusDisplay(
        color: Colors.green,
        icon: Icons.check_circle_outline,
        label: l10n.statusReady,
      ),
    // Фоллбэк: локализованная строка, НЕ сырой ключ из API.
    // ⚠️ Если задача 10.1 добавит новый статус — он попадёт сюда молча.
    // Тест project_status_display_test.dart проверяет все статусы из projectStatuses.
    _ => ProjectStatusDisplay(
        color: cs.outline,
        icon: Icons.help_outline,
        label: l10n.statusUnknown,
      ),
  };
}
