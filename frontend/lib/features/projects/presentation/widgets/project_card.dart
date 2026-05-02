import 'package:flutter/material.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/features/projects/presentation/utils/project_status_display.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';

class ProjectCard extends StatelessWidget {
  const ProjectCard({required this.project, super.key});
  final ProjectModel project;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final statusLabel = projectStatusDisplay(context, project.status).label;
    final providerLabel = gitProviderDisplayLabel(context, project.gitProvider);
    final localeTag = Localizations.localeOf(context).toLanguageTag();
    final dateFormat = DateFormat.yMMMd(localeTag);

    return Card(
      clipBehavior: Clip.antiAlias,
      child: Semantics(
        button: true,
        label: '${project.name}, $statusLabel',
        child: InkWell(
          key: Key('project-card-${project.id}'),
          // Uri.encodeComponent — защита от спецсимволов в ID.
          // UUID содержит только [a-z0-9-], encoding здесь no-op, но явное требование
          // на случай если бэкенд сменит формат ID.
          onTap: () => context.push('/projects/${Uri.encodeComponent(project.id)}'),
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        project.name,
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: 8),
                    ProjectStatusChip(status: project.status),
                  ],
                ),
                if (project.description.isNotEmpty) ...[
                  const SizedBox(height: 8),
                  Text(
                    project.description,
                    style: theme.textTheme.bodyMedium,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                const SizedBox(height: 8),
                Row(
                  children: [
                    _GitProviderIcon(provider: project.gitProvider),
                    const SizedBox(width: 4),
                    Expanded(
                      child: Text(
                        providerLabel,
                        style: theme.textTheme.bodySmall,
                      ),
                    ),
                    Text(
                      dateFormat.format(project.updatedAt),
                      style: theme.textTheme.bodySmall,
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// Временное решение: Material Icons не содержит brand-иконок git-провайдеров.
/// TODO: заменить на SVG-ассеты или font_awesome_flutter при наличии дизайн-системы.
/// TODO(10.6): вынести в отдельный файл widgets/git_provider_icon.dart для переиспользования.
/// Перечень провайдеров из [gitProviders] в задаче 10.1; default-ветвь покрывает новые.
class _GitProviderIcon extends StatelessWidget {
  const _GitProviderIcon({required this.provider});
  final String provider;

  @override
  Widget build(BuildContext context) {
    final icon = switch (provider) {
      'github' => Icons.code,
      'gitlab' => Icons.merge_type,
      'bitbucket' => Icons.source,
      'local' => Icons.folder_open,
      _ => Icons.cloud_queue,
    };
    return Icon(icon, size: 16);
  }
}
