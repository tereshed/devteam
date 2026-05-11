import 'package:flutter/material.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/utils/project_settings_update_patch.dart';
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
      ],
    );
  }
}
