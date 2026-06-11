import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/projects/data/project_providers.dart';

/// Бейдж scope панели ассистента: «Глобальный чат» вне проекта либо имя
/// текущего проекта. Делает видимым поведение «scope следует маршруту»:
/// при уходе с проектных экранов чат автоматически переключается на
/// глобальную сессию, и без бейджа это выглядело как потеря контекста.
class AssistantScopeBadge extends ConsumerWidget {
  const AssistantScopeBadge({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AssistantScopeBadge');
    final theme = Theme.of(context);
    final projectId = ref.watch(activeProjectIdProvider);

    final String label;
    final IconData icon;
    if (projectId == null) {
      label = l10n.assistantScopeGlobal;
      icon = Icons.public;
    } else {
      // Имя проекта подтягиваем асинхронно; пока грузится/ошибка — короткий id.
      final asyncProject = ref.watch(projectProvider(projectId));
      final name = asyncProject.asData?.value.name;
      label = l10n.assistantScopeProject(
        name ?? projectId.substring(0, projectId.length < 8 ? projectId.length : 8),
      );
      icon = Icons.folder_outlined;
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(width: 4),
          Flexible(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
