import 'package:flutter/material.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/utils/project_settings_update_patch.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Вектор-коллекция и reindex (13.4).
class ProjectSettingsVectorSection extends StatelessWidget {
  const ProjectSettingsVectorSection({
    super.key,
    required this.vectorController,
    required this.project,
    required this.reindexDisabled,
    required this.onVectorChanged,
    required this.onReindex,
  });

  final TextEditingController vectorController;
  final ProjectModel project;
  final bool reindexDisabled;
  final VoidCallback onVectorChanged;
  final VoidCallback onReindex;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.projectSettingsSectionVector,
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        TextFormField(
          key: const ValueKey('project-settings-vector-collection'),
          controller: vectorController,
          decoration: InputDecoration(
            labelText: l10n.projectSettingsVectorCollectionLabel,
            hintText: l10n.projectSettingsVectorCollectionHint,
          ),
          onChanged: (_) => onVectorChanged(),
          validator: (value) {
            final t = value?.trim() ?? '';
            if (!isValidVectorCollectionName(t)) {
              return l10n.projectSettingsVectorCollectionInvalid;
            }
            return null;
          },
        ),
        const SizedBox(height: 8),
        FilledButton.tonalIcon(
          onPressed: reindexDisabled ? null : onReindex,
          icon: const Icon(Icons.refresh),
          label: Text(
            project.status == 'indexing'
                ? l10n.projectSettingsReindexInProgress
                : l10n.projectSettingsReindex,
          ),
        ),
        if (reindexDisabled &&
            (project.gitProvider == kLocalGitProvider ||
                project.gitUrl.trim().isEmpty)) ...[
          const SizedBox(height: 4),
          Text(
            l10n.projectSettingsReindexUnavailable,
            style: theme.textTheme.bodySmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
        ],
        if (project.gitProvider != kLocalGitProvider &&
            project.gitUrl.trim().isNotEmpty) ...[
          const SizedBox(height: 12),
          Card(
            margin: EdgeInsets.zero,
            elevation: 0,
            color: theme.colorScheme.surfaceContainerLow,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(8),
              side: BorderSide(
                color: theme.colorScheme.outlineVariant.withValues(alpha: 0.5),
              ),
            ),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        l10n.projectSettingsIndexingStatusLabel,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      ProjectStatusChip(status: project.status),
                    ],
                  ),
                  if (project.lastIndexedCommit.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    const Divider(height: 1),
                    const SizedBox(height: 8),
                    Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          l10n.projectSettingsLastIndexedCommitLabel,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: theme.colorScheme.onSurfaceVariant,
                          ),
                        ),
                        const SizedBox(height: 4),
                        SelectableText(
                          project.lastIndexedCommit,
                          style: theme.textTheme.bodyMedium?.copyWith(
                            fontFamily: 'monospace',
                            color: theme.colorScheme.primary,
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
          ),
        ],
      ],
    );
  }
}
