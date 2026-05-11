import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/presentation/controllers/project_settings_controller.dart';
import 'package:frontend/features/projects/presentation/utils/project_settings_update_patch.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_git_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_tech_field_row.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_tech_stack_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_vector_section.dart';
import 'package:frontend/l10n/app_localizations.dart';

TextStyle? _projectSettingsSnackBarDetailStyle(ThemeData theme) {
  return theme.textTheme.bodySmall?.copyWith(
    color: theme.snackBarTheme.contentTextStyle?.color ??
        theme.colorScheme.onInverseSurface,
  );
}

/// Вкладка «Настройки» проекта: git, вектор, tech stack, reindex (13.4).
class ProjectSettingsScreen extends ConsumerStatefulWidget {
  const ProjectSettingsScreen({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<ProjectSettingsScreen> createState() =>
      _ProjectSettingsScreenState();
}

class _ProjectSettingsScreenState extends ConsumerState<ProjectSettingsScreen> {
  final _formKey = GlobalKey<FormState>();
  final _urlCtrl = TextEditingController();
  final _branchCtrl = TextEditingController();
  final _vectorCtrl = TextEditingController();

  String _gitProvider = gitProviders.first;
  final List<ProjectSettingsTechFieldRow> _techRows = [];
  bool _dirty = false;
  bool _pendingRemoveGitCredential = false;
  bool _explicitClearTechStack = false;
  bool _showVectorRenameWarning = false;
  String? _vectorSnapshotBeforeEdit;
  ProjectModel? _lastApplied;

  bool _reindexBusy = false;
  bool _saveBusy = false;
  bool _suppressNextProviderListenApply = false;

  @override
  void initState() {
    super.initState();
    assert(widget.projectId.isNotEmpty);
  }

  @override
  void dispose() {
    _urlCtrl.dispose();
    _branchCtrl.dispose();
    _vectorCtrl.dispose();
    for (final r in _techRows) {
      r.dispose();
    }
    super.dispose();
  }

  @override
  void didUpdateWidget(covariant ProjectSettingsScreen old) {
    super.didUpdateWidget(old);
    if (old.projectId != widget.projectId) {
      _resetForNewProject();
    }
  }

  void _resetForNewProject() {
    _urlCtrl.clear();
    _branchCtrl.clear();
    _vectorCtrl.clear();
    setState(() {
      _dirty = false;
      _pendingRemoveGitCredential = false;
      _explicitClearTechStack = false;
      _showVectorRenameWarning = false;
      _vectorSnapshotBeforeEdit = null;
      _lastApplied = null;
      _gitProvider = gitProviders.first;
      for (final r in _techRows) {
        r.dispose();
      }
      _techRows.clear();
      _techRows.add(ProjectSettingsTechFieldRow());
    });
  }

  void _applyFromProject(ProjectModel p) {
    _gitProvider = p.gitProvider;
    _urlCtrl.text = p.gitUrl;
    _branchCtrl.text = p.gitDefaultBranch;
    _vectorCtrl.text = p.vectorCollection;
    _vectorSnapshotBeforeEdit = p.vectorCollection.trim();
    for (final r in _techRows) {
      r.dispose();
    }
    _techRows.clear();
    final tech = projectBaselineTechStackStrings(p);
    if (tech.isEmpty) {
      _techRows.add(ProjectSettingsTechFieldRow());
    } else {
      for (final e in tech.entries) {
        _techRows.add(
          ProjectSettingsTechFieldRow(keyText: e.key, valueText: e.value),
        );
      }
    }
    _pendingRemoveGitCredential = false;
    _explicitClearTechStack = false;
    _lastApplied = p;
  }

  Map<String, String> _collectTechMap() {
    final out = <String, String>{};
    for (final r in _techRows) {
      final k = r.keyCtrl.text.trim();
      if (k.isEmpty) {
        continue;
      }
      out[k] = r.valueCtrl.text.trim();
    }
    return out;
  }

  void _markDirty() {
    if (!_dirty) {
      setState(() => _dirty = true);
    }
  }

  Future<void> _onSave(ProjectModel baseline) async {
    final messenger = ScaffoldMessenger.of(context);
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    if (_saveBusy) {
      return;
    }
    if (!_formKey.currentState!.validate()) {
      return;
    }

    final patch = buildProjectSettingsUpdateRequest(
      baseline: baseline,
      gitProvider: _gitProvider,
      gitUrl: _urlCtrl.text,
      gitDefaultBranch: _branchCtrl.text,
      vectorCollection: _vectorCtrl.text,
      techStackEditedNonEmptyKeys: _collectTechMap(),
      pendingRemoveGitCredential: _pendingRemoveGitCredential,
      explicitClearTechStack: _explicitClearTechStack,
    );

    if (patch == null || patch.toJson().isEmpty) {
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.projectSettingsNoChanges)),
      );
      return;
    }

    final prevVector = _vectorSnapshotBeforeEdit ?? baseline.vectorCollection.trim();
    setState(() => _saveBusy = true);
    try {
      final updated = await ref
          .read(projectSettingsControllerProvider(widget.projectId).notifier)
          .save(patch);
      if (!mounted || updated == null) {
        return;
      }
      setState(() {
        _dirty = false;
        _pendingRemoveGitCredential = false;
        _explicitClearTechStack = false;
        _applyFromProject(updated);
        final newV = updated.vectorCollection.trim();
        _showVectorRenameWarning =
            newV != prevVector && patch.vectorCollection != null;
        _suppressNextProviderListenApply = true;
      });
      ref.invalidate(projectProvider(widget.projectId));
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.projectSettingsSaved)),
      );
    } on Object catch (e) {
      if (!mounted) {
        return;
      }
      final title = projectSettingsSaveErrorTitle(l10n, e);
      final detail = projectSettingsErrorDetail(e);
      final detailStyle = _projectSettingsSnackBarDetailStyle(theme);
      messenger.showSnackBar(
        SnackBar(
          content: detail != null
              ? Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title),
                    Text(detail, style: detailStyle),
                  ],
                )
              : Text(title),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _saveBusy = false);
      }
    }
  }

  Future<void> _onReindex() async {
    final messenger = ScaffoldMessenger.of(context);
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    setState(() => _reindexBusy = true);
    try {
      await ref
          .read(projectSettingsControllerProvider(widget.projectId).notifier)
          .reindex();
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.projectSettingsReindexStarted)),
      );
    } on Object catch (e) {
      if (!mounted) {
        return;
      }
      final title = projectSettingsReindexErrorTitle(l10n, e);
      final detail = projectSettingsErrorDetail(e);
      final detailStyle = _projectSettingsSnackBarDetailStyle(theme);
      messenger.showSnackBar(
        SnackBar(
          content: detail != null
              ? Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title),
                    Text(detail, style: detailStyle),
                  ],
                )
              : Text(title),
        ),
      );
    } finally {
      if (mounted) {
        setState(() => _reindexBusy = false);
      }
    }
  }

  void _addTechRow() {
    setState(() {
      _dirty = true;
      _techRows.add(ProjectSettingsTechFieldRow());
    });
  }

  void _removeTechRow(int index) {
    setState(() {
      _dirty = true;
      _techRows[index].dispose();
      _techRows.removeAt(index);
      if (_techRows.isEmpty) {
        _techRows.add(ProjectSettingsTechFieldRow());
      }
    });
  }

  void _onClearTechStack() {
    setState(() {
      _explicitClearTechStack = true;
      _dirty = true;
      for (final r in _techRows) {
        r.dispose();
      }
      _techRows
        ..clear()
        ..add(ProjectSettingsTechFieldRow());
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final asyncProject = ref.watch(projectProvider(widget.projectId));

    ref.listen(projectProvider(widget.projectId), (prev, next) {
      if (next is! AsyncData<ProjectModel>) {
        return;
      }
      final value = next.value;
      if (_suppressNextProviderListenApply) {
        _suppressNextProviderListenApply = false;
        return;
      }
      if (!_dirty &&
          (_lastApplied?.id != value.id ||
              _lastApplied?.updatedAt != value.updatedAt)) {
        setState(() => _applyFromProject(value));
      }
    });

    if (asyncProject.hasError) {
      final err = asyncProject.error!;
      if (err is ProjectNotFoundException) {
        return const SizedBox.shrink();
      }
      return DataLoadErrorMessage(
        title: l10n.dataLoadError,
        actionLabel: l10n.retry,
        onAction: () => ref.invalidate(projectProvider(widget.projectId)),
      );
    }

    if (asyncProject.isLoading || !asyncProject.hasValue) {
      return const Center(child: CircularProgressIndicator());
    }

    final project = asyncProject.requireValue;

    final reindexDisabled =
        project.status == 'indexing' ||
        project.gitProvider == kLocalGitProvider ||
        project.gitUrl.trim().isEmpty ||
        _reindexBusy;

    Future<void> onRefresh() async {
      ref.invalidate(projectProvider(widget.projectId));
      try {
        await ref.read(projectProvider(widget.projectId).future);
      } on Exception {
        // asyncProject уже покажет ошибку
      }
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (_showVectorRenameWarning)
          MaterialBanner(
            content: Text(l10n.projectSettingsVectorCollectionRenamed),
            backgroundColor: theme.colorScheme.surfaceContainerHighest,
            actions: [
              TextButton(
                onPressed: () =>
                    setState(() => _showVectorRenameWarning = false),
                child: Text(MaterialLocalizations.of(context).closeButtonLabel),
              ),
            ],
          ),
        Expanded(
          child: RefreshIndicator(
            onRefresh: onRefresh,
            child: SingleChildScrollView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
              child: Form(
                key: _formKey,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    ProjectSettingsGitSection(
                      gitProvider: _gitProvider,
                      onGitProviderChanged: (v) {
                        setState(() {
                          _gitProvider = v;
                          _dirty = true;
                        });
                      },
                      urlController: _urlCtrl,
                      branchController: _branchCtrl,
                      project: project,
                      pendingRemoveGitCredential: _pendingRemoveGitCredential,
                      onToggleUnlinkCredential: () {
                        setState(() {
                          _pendingRemoveGitCredential =
                              !_pendingRemoveGitCredential;
                          _dirty = true;
                        });
                      },
                      onFieldChanged: _markDirty,
                    ),
                    const SizedBox(height: 24),
                    ProjectSettingsVectorSection(
                      vectorController: _vectorCtrl,
                      project: project,
                      reindexDisabled: reindexDisabled,
                      onVectorChanged: _markDirty,
                      onReindex: _onReindex,
                    ),
                    const SizedBox(height: 24),
                    ProjectSettingsTechStackSection(
                      rows: _techRows,
                      onAddRow: _addTechRow,
                      onRemoveRow: _removeTechRow,
                      onClearTechStack: _onClearTechStack,
                      onRowChanged: _markDirty,
                    ),
                    const SizedBox(height: 24),
                    FilledButton(
                      onPressed: _saveBusy ? null : () => _onSave(project),
                      child: Text(l10n.projectSettingsSave),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}
