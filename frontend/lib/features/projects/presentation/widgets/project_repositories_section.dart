import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/models/project_repository_model.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/features/projects/presentation/widgets/git_repo_source_selector.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Секция управления git-репозиториями проекта (мульти-репо).
///
/// Самодостаточна: сама watch'ит [projectRepositoriesProvider], выполняет add/remove
/// и инвалидирует провайдер после мутаций.
class ProjectRepositoriesSection extends ConsumerWidget {
  const ProjectRepositoriesSection({super.key, required this.projectId});

  final String projectId;

  Future<void> _refresh(WidgetRef ref) async {
    ref.invalidate(projectRepositoriesProvider(projectId));
    // projectProvider тоже содержит repositories (GET /projects/:id) — обновим карточку.
    ref.invalidate(projectProvider(projectId));
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final asyncRepos = ref.watch(projectRepositoriesProvider(projectId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                l10n.repositoriesSectionTitle,
                style: theme.textTheme.titleMedium,
              ),
            ),
            TextButton.icon(
              onPressed: () => _openAddDialog(context, ref),
              icon: const Icon(Icons.add, size: 18),
              label: Text(l10n.repositoriesAddButton),
            ),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          l10n.repositoriesSectionSubtitle,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 12),
        asyncRepos.when(
          loading: () => const Padding(
            padding: EdgeInsets.symmetric(vertical: 16),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (e, _) => DataLoadErrorMessage(
            title: l10n.dataLoadError,
            actionLabel: l10n.retry,
            onAction: () => ref.invalidate(projectRepositoriesProvider(projectId)),
          ),
          data: (repos) {
            if (repos.isEmpty) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Text(
                  l10n.repositoriesEmpty,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              );
            }
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                for (final repo in repos)
                  _RepositoryTile(
                    repo: repo,
                    canRemove: repos.length > 1 || !repo.isPrimary,
                    onRemove: () => _confirmRemove(context, ref, repo),
                  ),
              ],
            );
          },
        ),
      ],
    );
  }

  Future<void> _confirmRemove(
    BuildContext context,
    WidgetRef ref,
    ProjectRepositoryModel repo,
  ) async {
    final l10n = AppLocalizations.of(context)!;
    final messenger = ScaffoldMessenger.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.repositoryRemoveConfirmTitle),
        content: Text(l10n.repositoryRemoveConfirmBody(repo.slug)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(MaterialLocalizations.of(ctx).cancelButtonLabel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.repositoryRemoveConfirmAction),
          ),
        ],
      ),
    );
    if (confirmed != true) {
      return;
    }
    try {
      await ref
          .read(projectRepositoryProvider)
          .deleteRepository(projectId, repo.id);
      await _refresh(ref);
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _openAddDialog(BuildContext context, WidgetRef ref) async {
    final messenger = ScaffoldMessenger.of(context);
    final request = await showDialog<CreateRepositoryRequest>(
      context: context,
      builder: (ctx) => const _AddRepositoryDialog(),
    );
    if (request == null) {
      return;
    }
    try {
      await ref
          .read(projectRepositoryProvider)
          .createRepository(projectId, request);
      await _refresh(ref);
    } on Object catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }
}

class _RepositoryTile extends StatelessWidget {
  const _RepositoryTile({
    required this.repo,
    required this.canRemove,
    required this.onRemove,
  });

  final ProjectRepositoryModel repo;
  final bool canRemove;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  repo.slug,
                  style: theme.textTheme.titleSmall,
                ),
                const SizedBox(width: 8),
                if (repo.isPrimary)
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.primary.withValues(alpha: 0.12),
                      borderRadius: BorderRadius.circular(4),
                    ),
                    child: Text(
                      l10n.repositoryPrimaryBadge,
                      style: theme.textTheme.labelSmall
                          ?.copyWith(color: theme.colorScheme.primary),
                    ),
                  ),
                const Spacer(),
                ProjectStatusChip(status: repo.status),
                if (canRemove)
                  IconButton(
                    tooltip: l10n.repositoryRemoveTooltip,
                    icon: const Icon(Icons.delete_outline, size: 20),
                    onPressed: onRemove,
                  ),
              ],
            ),
            const SizedBox(height: 4),
            Text(
              repo.displayName,
              style: theme.textTheme.bodyMedium,
            ),
            Text(
              '${gitProviderDisplayLabel(context, repo.gitProvider)} · ${repo.gitUrl}',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
            if (repo.roleDescription.trim().isNotEmpty) ...[
              const SizedBox(height: 4),
              Text(
                repo.roleDescription,
                style: theme.textTheme.bodySmall,
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _AddRepositoryDialog extends StatefulWidget {
  const _AddRepositoryDialog();

  @override
  State<_AddRepositoryDialog> createState() => _AddRepositoryDialogState();
}

class _AddRepositoryDialogState extends State<_AddRepositoryDialog> {
  final _formKey = GlobalKey<FormState>();
  final _slugCtrl = TextEditingController();
  final _nameCtrl = TextEditingController();
  final _branchCtrl = TextEditingController(text: 'main');
  final _roleCtrl = TextEditingController();

  // Источник репо (провайдер/аккаунт/URL) из GitRepoSourceSelector.
  GitRepoSource _source = const GitRepoSource(
    gitProvider: kLocalGitProvider,
    accountId: null,
    gitUrl: '',
  );

  // Clone URL репо, по которому уже выполнили автозаполнение — чтобы не затирать
  // ручные правки на каждый rebuild, но перезаполнять при выборе другого репо.
  String? _autofilledUrl;

  @override
  void dispose() {
    _slugCtrl.dispose();
    _nameCtrl.dispose();
    _branchCtrl.dispose();
    _roleCtrl.dispose();
    super.dispose();
  }

  /// Автозаполнение полей из метаданных выбранного репозитория: slug, отображаемое
  /// имя, ветка по умолчанию, описание. Срабатывает один раз на каждый новый выбор.
  void _onSourceChanged(GitRepoSource src) {
    final repo = src.repo;
    if (repo != null && repo.cloneUrl != _autofilledUrl) {
      _autofilledUrl = repo.cloneUrl;
      _slugCtrl.text = _slugify(repo.name);
      _nameCtrl.text = repo.name;
      _branchCtrl.text =
          repo.defaultBranch.isNotEmpty ? repo.defaultBranch : 'main';
      final desc = repo.description?.trim() ?? '';
      if (desc.isNotEmpty) {
        _roleCtrl.text = desc;
      }
    }
    setState(() => _source = src);
  }

  /// repo name → git-slug: латиница/цифры, остальное → '-', обрезка до 64 символов.
  static String _slugify(String s) {
    final slug = s
        .trim()
        .toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9]+'), '-')
        .replaceAll(RegExp(r'^-+|-+$'), '');
    return slug.length > 64 ? slug.substring(0, 64) : slug;
  }

  void _submit() {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    final slug = _slugCtrl.text.trim();
    Navigator.of(context).pop(
      CreateRepositoryRequest(
        slug: slug,
        displayName:
            _nameCtrl.text.trim().isEmpty ? slug : _nameCtrl.text.trim(),
        roleDescription: _roleCtrl.text.trim(),
        gitProvider: _source.gitProvider,
        gitUrl: _source.gitUrl,
        gitDefaultBranch:
            _branchCtrl.text.trim().isEmpty ? 'main' : _branchCtrl.text.trim(),
        gitIntegrationCredentialId: _source.accountId,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return AlertDialog(
      title: Text(l10n.repositoryAddDialogTitle),
      content: SizedBox(
        width: 460,
        child: Form(
          key: _formKey,
          child: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              mainAxisSize: MainAxisSize.min,
              children: [
                // Выбор аккаунта → репозитория из списка (как в форме создания проекта).
                // Поля ниже автозаполняются из выбранного репо.
                GitRepoSourceSelector(onChanged: _onSourceChanged),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _slugCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.repositoryFieldSlug,
                    hintText: l10n.repositoryFieldSlugHint,
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? '' : null,
                ),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _nameCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.repositoryFieldDisplayName,
                  ),
                ),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _branchCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.repositoryFieldBranch,
                  ),
                ),
                const SizedBox(height: 8),
                TextFormField(
                  controller: _roleCtrl,
                  minLines: 1,
                  maxLines: 3,
                  decoration: InputDecoration(
                    labelText: l10n.repositoryFieldRole,
                    hintText: l10n.repositoryFieldRoleHint,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(MaterialLocalizations.of(context).cancelButtonLabel),
        ),
        FilledButton(
          onPressed: _submit,
          child: Text(l10n.repositoryAddSubmit),
        ),
      ],
    );
  }
}
