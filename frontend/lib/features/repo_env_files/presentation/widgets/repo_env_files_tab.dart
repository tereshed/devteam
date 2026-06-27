import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_repository_model.dart';
import 'package:frontend/features/repo_env_files/data/repo_env_file_providers.dart';
import 'package:frontend/features/repo_env_files/domain/models/repo_env_file_model.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Вкладка «Env-файлы» в настройках проекта: «инъекция env-файла» уровня репозитория.
/// Пользователь выбирает репозиторий и задаёт один файл (содержимое, имя, папку),
/// который sandbox пишет в рабочую копию репо и исключает из git.
class RepoEnvFilesTab extends ConsumerStatefulWidget {
  const RepoEnvFilesTab({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<RepoEnvFilesTab> createState() => _RepoEnvFilesTabState();
}

class _RepoEnvFilesTabState extends ConsumerState<RepoEnvFilesTab> {
  String? _selectedRepoId;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'RepoEnvFilesTab');
    final reposAsync = ref.watch(projectRepositoriesProvider(widget.projectId));

    return SingleChildScrollView(
      physics: const AlwaysScrollableScrollPhysics(),
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.repoEnvFilesHeading, style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 8),
          Text(
            l10n.repoEnvFilesDescription,
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 20),
          reposAsync.when(
            loading: () => const Center(child: Padding(
              padding: EdgeInsets.all(24),
              child: CircularProgressIndicator(),
            )),
            error: (e, _) => Text(l10n.repoEnvFilesLoadError),
            data: (repos) {
              if (repos.isEmpty) {
                return Text(l10n.repoEnvFilesNoRepos);
              }
              final selectedId = _resolveSelectedId(repos);
              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  DropdownButtonFormField<String>(
                    initialValue: selectedId,
                    decoration: InputDecoration(
                      labelText: l10n.repoEnvFilesSelectRepo,
                      border: const OutlineInputBorder(),
                    ),
                    items: repos
                        .map((r) => DropdownMenuItem(
                              value: r.id,
                              child: Text(_repoLabel(r)),
                            ))
                        .toList(),
                    onChanged: (v) => setState(() => _selectedRepoId = v),
                  ),
                  const SizedBox(height: 20),
                  _buildFormForRepo(context, selectedId),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  String _resolveSelectedId(List<ProjectRepositoryModel> repos) {
    final current = _selectedRepoId;
    if (current != null && repos.any((r) => r.id == current)) {
      return current;
    }
    final primary = repos.firstWhere(
      (r) => r.isPrimary,
      orElse: () => repos.first,
    );
    return primary.id;
  }

  String _repoLabel(ProjectRepositoryModel r) {
    final name = r.displayName.isNotEmpty ? r.displayName : r.slug;
    return r.isPrimary ? '$name ★' : name;
  }

  Widget _buildFormForRepo(BuildContext context, String repoId) {
    final l10n = requireAppLocalizations(context, where: 'RepoEnvFilesTab');
    final fileAsync = ref.watch(repoEnvFileProvider(widget.projectId, repoId));
    return fileAsync.when(
      loading: () => const Center(child: Padding(
        padding: EdgeInsets.all(24),
        child: CircularProgressIndicator(),
      )),
      error: (e, _) => Text(l10n.repoEnvFilesLoadError),
      data: (model) => _RepoEnvFileForm(
        key: ValueKey(repoId),
        projectId: widget.projectId,
        repoId: repoId,
        initial: model,
      ),
    );
  }
}

/// Форма редактирования env-файла одного репозитория. Пересоздаётся (ValueKey по repoId)
/// при смене репозитория — initState сидит контроллеры из загруженной модели.
class _RepoEnvFileForm extends ConsumerStatefulWidget {
  const _RepoEnvFileForm({
    super.key,
    required this.projectId,
    required this.repoId,
    required this.initial,
  });

  final String projectId;
  final String repoId;
  final RepoEnvFileModel? initial;

  @override
  ConsumerState<_RepoEnvFileForm> createState() => _RepoEnvFileFormState();
}

class _RepoEnvFileFormState extends ConsumerState<_RepoEnvFileForm> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _fileNameCtrl;
  late final TextEditingController _targetDirCtrl;
  late final TextEditingController _contentCtrl;
  bool _busy = false;

  bool get _exists => widget.initial != null;

  @override
  void initState() {
    super.initState();
    final m = widget.initial;
    // Имя/папка не секретны — префиллим. Содержимое write-only: НЕ возвращается с бэка,
    // поле всегда пустое; сохранение перезаписывает файл целиком.
    _fileNameCtrl = TextEditingController(text: m?.fileName ?? '.env');
    _targetDirCtrl = TextEditingController(text: m?.targetDir ?? '');
    _contentCtrl = TextEditingController();
  }

  @override
  void dispose() {
    _fileNameCtrl.dispose();
    _targetDirCtrl.dispose();
    _contentCtrl.dispose();
    super.dispose();
  }

  Future<void> _onSave() async {
    final l10n = requireAppLocalizations(context, where: '_RepoEnvFileForm');
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _busy = true);
    try {
      await ref.read(repoEnvFileRepositoryProvider).set(
            widget.projectId,
            widget.repoId,
            fileName: _fileNameCtrl.text.trim(),
            targetDir: _targetDirCtrl.text.trim(),
            content: _contentCtrl.text,
          );
      ref.invalidate(repoEnvFileProvider(widget.projectId, widget.repoId));
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaved)));
    } on Object {
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaveError)));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _onDelete() async {
    final l10n = requireAppLocalizations(context, where: '_RepoEnvFileForm');
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        content: Text(l10n.repoEnvFilesDeleteConfirm),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(MaterialLocalizations.of(ctx).cancelButtonLabel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.repoEnvFilesDelete),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _busy = true);
    try {
      await ref.read(repoEnvFileRepositoryProvider).delete(widget.projectId, widget.repoId);
      ref.invalidate(repoEnvFileProvider(widget.projectId, widget.repoId));
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesDeleted)));
    } on Object {
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaveError)));
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  /// Баннер статуса: настроен ли файл (зелёная галочка + дата последнего сохранения)
  /// или ещё нет. Даёт явное подтверждение после «Сохранить».
  Widget _buildStatusBanner(BuildContext context, AppLocalizations l10n) {
    final theme = Theme.of(context);
    final cs = theme.colorScheme;
    final configured = _exists;
    final bg = configured
        ? cs.primaryContainer.withValues(alpha: 0.5)
        : cs.surfaceContainerHighest;
    final fg = configured ? cs.onPrimaryContainer : cs.onSurfaceVariant;

    final updated = _formatTimestamp(widget.initial?.updatedAt);
    final message = configured
        ? (updated.isEmpty
            ? l10n.repoEnvFilesConfiguredHidden
            : '${l10n.repoEnvFilesConfiguredHidden} '
                '${l10n.repoEnvFilesUpdatedLabel} $updated')
        : l10n.repoEnvFilesNotConfigured;

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            configured ? Icons.check_circle : Icons.info_outline,
            size: 18,
            color: fg,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              message,
              style: theme.textTheme.bodySmall?.copyWith(color: fg),
            ),
          ),
        ],
      ),
    );
  }

  String _formatTimestamp(String? iso) {
    if (iso == null || iso.isEmpty) {
      return '';
    }
    final dt = DateTime.tryParse(iso)?.toLocal();
    if (dt == null) {
      return '';
    }
    String two(int n) => n.toString().padLeft(2, '0');
    return '${dt.year}-${two(dt.month)}-${two(dt.day)} ${two(dt.hour)}:${two(dt.minute)}';
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_RepoEnvFileForm');
    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _buildStatusBanner(context, l10n),
          const SizedBox(height: 12),
          TextFormField(
            controller: _fileNameCtrl,
            decoration: InputDecoration(
              labelText: l10n.repoEnvFilesFileNameLabel,
              hintText: l10n.repoEnvFilesFileNameHint,
              border: const OutlineInputBorder(),
            ),
            validator: (v) => (v == null || v.trim().isEmpty)
                ? l10n.repoEnvFilesValidationFileNameRequired
                : null,
          ),
          const SizedBox(height: 16),
          TextFormField(
            controller: _targetDirCtrl,
            decoration: InputDecoration(
              labelText: l10n.repoEnvFilesTargetDirLabel,
              hintText: l10n.repoEnvFilesTargetDirHint,
              border: const OutlineInputBorder(),
            ),
          ),
          const SizedBox(height: 16),
          TextFormField(
            controller: _contentCtrl,
            decoration: InputDecoration(
              labelText: l10n.repoEnvFilesContentLabel,
              hintText: l10n.repoEnvFilesContentHint,
              border: const OutlineInputBorder(),
              alignLabelWithHint: true,
            ),
            minLines: 6,
            maxLines: 20,
            style: const TextStyle(fontFamily: 'monospace'),
            validator: (v) => (v == null || v.isEmpty)
                ? l10n.repoEnvFilesValidationContentRequired
                : null,
          ),
          const SizedBox(height: 20),
          Row(
            children: [
              Expanded(
                child: FilledButton(
                  onPressed: _busy ? null : _onSave,
                  child: Text(l10n.repoEnvFilesSave),
                ),
              ),
              if (_exists) ...[
                const SizedBox(width: 12),
                OutlinedButton(
                  onPressed: _busy ? null : _onDelete,
                  child: Text(l10n.repoEnvFilesDelete),
                ),
              ],
            ],
          ),
        ],
      ),
    );
  }
}
