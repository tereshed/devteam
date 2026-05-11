import 'package:flutter/material.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/features/projects/presentation/utils/git_remote_url.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Git: провайдер, URL, ветка, карточка credential (13.4).
class ProjectSettingsGitSection extends StatelessWidget {
  const ProjectSettingsGitSection({
    super.key,
    required this.gitProvider,
    required this.onGitProviderChanged,
    required this.urlController,
    required this.branchController,
    required this.project,
    required this.pendingRemoveGitCredential,
    required this.onToggleUnlinkCredential,
    required this.onFieldChanged,
  });

  final String gitProvider;
  final ValueChanged<String> onGitProviderChanged;
  final TextEditingController urlController;
  final TextEditingController branchController;
  final ProjectModel project;
  final bool pendingRemoveGitCredential;
  final VoidCallback onToggleUnlinkCredential;
  final VoidCallback onFieldChanged;

  bool _isRemoteProvider(String p) => p != kLocalGitProvider;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.projectSettingsSectionGit,
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        DropdownButtonFormField<String>(
          // Controlled field: `initialValue` is one-shot; sync from [ProjectModel] via setState.
          // ignore: deprecated_member_use
          value: gitProvider,
          decoration: InputDecoration(
            labelText: l10n.gitProviderFieldLabel,
          ),
          items: [
            for (final p in gitProviders)
              DropdownMenuItem(
                value: p,
                child: Text(gitProviderDisplayLabel(context, p)),
              ),
          ],
          onChanged: (v) {
            if (v == null) {
              return;
            }
            onGitProviderChanged(v);
          },
        ),
        const SizedBox(height: 8),
        TextFormField(
          key: const ValueKey('project-settings-git-url'),
          controller: urlController,
          decoration: InputDecoration(
            labelText: l10n.gitUrlFieldLabel,
            hintText: l10n.gitUrlFieldHint,
          ),
          onChanged: (_) => onFieldChanged(),
          validator: (value) {
            if (!_isRemoteProvider(gitProvider)) {
              return null;
            }
            final t = value?.trim() ?? '';
            if (t.isEmpty) {
              return l10n.gitUrlRequiredForRemote;
            }
            if (!isValidGitRemoteUrl(t)) {
              return l10n.gitUrlInvalid;
            }
            return null;
          },
        ),
        const SizedBox(height: 8),
        TextFormField(
          key: const ValueKey('project-settings-git-branch'),
          controller: branchController,
          decoration: InputDecoration(
            labelText: l10n.projectSettingsGitDefaultBranchLabel,
          ),
          onChanged: (_) => onFieldChanged(),
        ),
        if (project.gitCredential != null) ...[
          const SizedBox(height: 12),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    l10n.projectSettingsGitCredentialCardTitle,
                    style: theme.textTheme.titleSmall,
                  ),
                  const SizedBox(height: 8),
                  Text(project.gitCredential!.label),
                  Text(
                    '${project.gitCredential!.provider} · ${project.gitCredential!.authType}',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  if (pendingRemoveGitCredential)
                    Padding(
                      padding: const EdgeInsets.only(top: 8),
                      child: Text(
                        l10n.projectSettingsUnlinkPendingHint,
                        style: theme.textTheme.bodySmall,
                      ),
                    ),
                  const SizedBox(height: 8),
                  OutlinedButton(
                    onPressed: onToggleUnlinkCredential,
                    child: Text(l10n.projectSettingsUnlinkCredential),
                  ),
                ],
              ),
            ),
          ),
        ],
      ],
    );
  }
}
