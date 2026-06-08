import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/widgets/git_account_dropdown.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Секция выбора OAuth-аккаунта провайдера для проекта (мульти-аккаунт).
/// Самодостаточна: сохраняет выбор сразу (PUT /projects/:id) и инвалидирует projectProvider.
class ProjectGitAccountSection extends ConsumerWidget {
  const ProjectGitAccountSection({super.key, required this.project});

  final ProjectModel project;

  static const _localProvider = 'local';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (project.gitProvider == _localProvider) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l10n.gitAccountSectionTitle, style: theme.textTheme.titleMedium),
        const SizedBox(height: 8),
        GitAccountDropdown(
          providerJsonValue: project.gitProvider,
          selectedId: project.gitIntegrationCredentialId,
          onChanged: (id) => _save(context, ref, id),
        ),
      ],
    );
  }

  Future<void> _save(BuildContext context, WidgetRef ref, String? id) async {
    if (id == project.gitIntegrationCredentialId) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(projectRepositoryProvider).updateProject(
            project.id,
            UpdateRepositoryAccountPatch.project(id),
          );
      ref.invalidate(projectProvider(project.id));
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }
}

/// Хелпер для частичного апдейта только git_integration_credential_id проекта.
abstract final class UpdateRepositoryAccountPatch {
  static UpdateProjectRequest project(String? id) {
    return UpdateProjectRequest(
      gitIntegrationCredentialId: id,
      removeGitIntegrationCredential: id == null,
    );
  }
}
