import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_repository_model.dart';
import 'package:frontend/features/repo_env_files/data/repo_env_file_providers.dart';
import 'package:frontend/features/repo_env_files/domain/models/repo_env_file_model.dart';

/// Вкладка «Env-файлы» в настройках проекта: «инъекция env-файлов» уровня репозитория.
/// Пользователь выбирает репозиторий и управляет НЕСКОЛЬКИМИ файлами (каждый — содержимое,
/// имя, папка), которые sandbox пишет в рабочую копию репо и исключает из git.
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
          Text(l10n.repoEnvFilesDescription, style: Theme.of(context).textTheme.bodySmall),
          const SizedBox(height: 20),
          reposAsync.when(
            loading: () => const Center(
              child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()),
            ),
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
                        .map((r) => DropdownMenuItem(value: r.id, child: Text(_repoLabel(r))))
                        .toList(),
                    onChanged: (v) => setState(() => _selectedRepoId = v),
                  ),
                  const SizedBox(height: 20),
                  _FilesList(projectId: widget.projectId, repoId: selectedId),
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
    final primary = repos.firstWhere((r) => r.isPrimary, orElse: () => repos.first);
    return primary.id;
  }

  String _repoLabel(ProjectRepositoryModel r) {
    final name = r.displayName.isNotEmpty ? r.displayName : r.slug;
    return r.isPrimary ? '$name ★' : name;
  }
}

/// Список env-файлов выбранного репозитория + кнопка добавления.
class _FilesList extends ConsumerWidget {
  const _FilesList({required this.projectId, required this.repoId});

  final String projectId;
  final String repoId;

  String _pathLabel(RepoEnvFileModel f) {
    if (f.targetDir.isEmpty) {
      return f.fileName;
    }
    return '${f.targetDir}/${f.fileName}';
  }

  Future<void> _openDialog(BuildContext context, WidgetRef ref, {RepoEnvFileModel? existing}) async {
    await showDialog<void>(
      context: context,
      builder: (_) => _RepoEnvFileDialog(
        projectId: projectId,
        repoId: repoId,
        existing: existing,
      ),
    );
  }

  Future<void> _delete(BuildContext context, WidgetRef ref, RepoEnvFileModel f) async {
    final l10n = requireAppLocalizations(context, where: '_FilesList');
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
    if (confirmed != true || !context.mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(repoEnvFileRepositoryProvider).delete(projectId, repoId, f.id);
      ref.invalidate(repoEnvFilesProvider(projectId, repoId));
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesDeleted)));
    } on Object {
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaveError)));
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: '_FilesList');
    final filesAsync = ref.watch(repoEnvFilesProvider(projectId, repoId));

    return filesAsync.when(
      loading: () => const Center(
        child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()),
      ),
      error: (e, _) => Text(l10n.repoEnvFilesLoadError),
      data: (files) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (files.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 8),
                child: Text(l10n.repoEnvFilesEmpty, style: Theme.of(context).textTheme.bodySmall),
              )
            else
              ...files.map(
                (f) => Card(
                  margin: const EdgeInsets.only(bottom: 8),
                  child: ListTile(
                    leading: const Icon(Icons.description_outlined),
                    title: Text(_pathLabel(f)),
                    subtitle: f.updatedAt != null
                        ? Text('${l10n.repoEnvFilesUpdatedLabel} ${_formatTimestamp(f.updatedAt)}')
                        : null,
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        IconButton(
                          icon: const Icon(Icons.edit_outlined),
                          tooltip: l10n.repoEnvFilesEditTitle,
                          onPressed: () => _openDialog(context, ref, existing: f),
                        ),
                        IconButton(
                          icon: const Icon(Icons.delete_outline),
                          tooltip: l10n.repoEnvFilesDelete,
                          onPressed: () => _delete(context, ref, f),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            const SizedBox(height: 8),
            Align(
              alignment: Alignment.centerLeft,
              child: OutlinedButton.icon(
                onPressed: () => _openDialog(context, ref),
                icon: const Icon(Icons.add),
                label: Text(l10n.repoEnvFilesAddButton),
              ),
            ),
          ],
        );
      },
    );
  }
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

/// Диалог создания/редактирования env-файла. Содержимое write-only: при редактировании
/// поле пустое, сохранение перезаписывает файл целиком.
class _RepoEnvFileDialog extends ConsumerStatefulWidget {
  const _RepoEnvFileDialog({
    required this.projectId,
    required this.repoId,
    required this.existing,
  });

  final String projectId;
  final String repoId;
  final RepoEnvFileModel? existing;

  @override
  ConsumerState<_RepoEnvFileDialog> createState() => _RepoEnvFileDialogState();
}

class _RepoEnvFileDialogState extends ConsumerState<_RepoEnvFileDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _fileNameCtrl;
  late final TextEditingController _targetDirCtrl;
  final _contentCtrl = TextEditingController();
  bool _busy = false;

  bool get _isEdit => widget.existing != null;

  @override
  void initState() {
    super.initState();
    _fileNameCtrl = TextEditingController(text: widget.existing?.fileName ?? '.env');
    _targetDirCtrl = TextEditingController(text: widget.existing?.targetDir ?? '');
  }

  @override
  void dispose() {
    _fileNameCtrl.dispose();
    _targetDirCtrl.dispose();
    _contentCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l10n = requireAppLocalizations(context, where: '_RepoEnvFileDialog');
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    final navigator = Navigator.of(context);
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _busy = true);
    try {
      final repo = ref.read(repoEnvFileRepositoryProvider);
      if (_isEdit) {
        await repo.update(
          widget.projectId,
          widget.repoId,
          widget.existing!.id,
          fileName: _fileNameCtrl.text.trim(),
          targetDir: _targetDirCtrl.text.trim(),
          content: _contentCtrl.text,
        );
      } else {
        await repo.create(
          widget.projectId,
          widget.repoId,
          fileName: _fileNameCtrl.text.trim(),
          targetDir: _targetDirCtrl.text.trim(),
          content: _contentCtrl.text,
        );
      }
      ref.invalidate(repoEnvFilesProvider(widget.projectId, widget.repoId));
      navigator.pop();
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaved)));
    } on Object {
      if (mounted) {
        setState(() => _busy = false);
      }
      messenger.showSnackBar(SnackBar(content: Text(l10n.repoEnvFilesSaveError)));
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_RepoEnvFileDialog');
    return AlertDialog(
      title: Text(_isEdit ? l10n.repoEnvFilesEditTitle : l10n.repoEnvFilesCreateTitle),
      content: SizedBox(
        width: 520,
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
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
                if (_isEdit)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: Text(
                      l10n.repoEnvFilesConfiguredHidden,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                TextFormField(
                  controller: _contentCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.repoEnvFilesContentLabel,
                    hintText: l10n.repoEnvFilesContentHint,
                    border: const OutlineInputBorder(),
                    alignLabelWithHint: true,
                  ),
                  minLines: 6,
                  maxLines: 16,
                  style: const TextStyle(fontFamily: 'monospace'),
                  validator: (v) => (v == null || v.isEmpty)
                      ? l10n.repoEnvFilesValidationContentRequired
                      : null,
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(MaterialLocalizations.of(context).cancelButtonLabel),
        ),
        FilledButton(
          onPressed: _busy ? null : _submit,
          child: Text(l10n.repoEnvFilesSave),
        ),
      ],
    );
  }
}
