import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/assistant_mcp/presentation/widgets/assistant_mcp_tab.dart';
import 'package:frontend/features/enhancer/presentation/widgets/enhancer_settings_tab.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/models/project_model.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/controllers/project_settings_controller.dart';
import 'package:frontend/features/projects/presentation/utils/project_settings_update_patch.dart';
import 'package:frontend/features/projects/presentation/widgets/project_git_account_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_repositories_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_git_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_tech_field_row.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_tech_stack_section.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_vector_section.dart';
import 'package:frontend/features/repo_env_files/presentation/widgets/repo_env_files_tab.dart';
import 'package:frontend/features/sandbox_services/presentation/widgets/sandbox_services_tab.dart';
import 'package:frontend/features/scout/presentation/widgets/scout_settings_tab.dart';
import 'package:frontend/features/settings/presentation/widgets/assistant_prompt_editor.dart';
import 'package:frontend/features/team/presentation/widgets/project_variables_section.dart';
import 'package:frontend/features/webhooks/presentation/widgets/webhooks_list_section.dart';
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
  final _branchTemplateCtrl = TextEditingController();
  final _branchPatternCtrl = TextEditingController();
  final _mrTitleCtrl = TextEditingController();
  final _vectorCtrl = TextEditingController();

  String _gitProvider = gitProviders.first;
  bool _branchNamingLocked = false;
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
    _branchTemplateCtrl.dispose();
    _branchPatternCtrl.dispose();
    _mrTitleCtrl.dispose();
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
    _branchTemplateCtrl.clear();
    _branchPatternCtrl.clear();
    _mrTitleCtrl.clear();
    _vectorCtrl.clear();
    setState(() {
      _dirty = false;
      _pendingRemoveGitCredential = false;
      _explicitClearTechStack = false;
      _showVectorRenameWarning = false;
      _vectorSnapshotBeforeEdit = null;
      _lastApplied = null;
      _gitProvider = gitProviders.first;
      _branchNamingLocked = false;
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
    _branchTemplateCtrl.text = p.branchNameTemplate ?? '';
    _branchPatternCtrl.text = p.branchNamePattern ?? '';
    _mrTitleCtrl.text = p.mrTitleTemplate ?? '';
    _branchNamingLocked = p.branchNamingLocked;
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
    final l10n = requireAppLocalizations(
      context,
      where: 'projectSettingsScreen.save',
    );
    final theme = Theme.of(context);
    if (_saveBusy) {
      return;
    }
    // Форма живёт в табе General; tech stack вынесен в отдельный таб (вне формы),
    // поэтому при сохранении оттуда _formKey.currentState может быть null (таб
    // General не смонтирован). Null трактуем как «валидно» — у tech stack нет
    // валидаторов, а git/vector берутся из контроллеров уровня экрана.
    if (!(_formKey.currentState?.validate() ?? true)) {
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
      branchNameTemplate: _branchTemplateCtrl.text,
      branchNamePattern: _branchPatternCtrl.text,
      branchNamingLocked: _branchNamingLocked,
      mrTitleTemplate: _mrTitleCtrl.text,
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
    final l10n = requireAppLocalizations(
      context,
      where: 'projectSettingsScreen.reindex',
    );
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
    // Watch settings controller to keep its CancelTokens alive as long as this screen is mounted.
    ref.watch(projectSettingsControllerProvider(widget.projectId));

    final l10n = requireAppLocalizations(
      context,
      where: 'projectSettingsScreen.body',
    );
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

    // ref.listen выше срабатывает только на ПЕРЕХОДЫ провайдера. Если данные
    // уже были в кеше Riverpod (типичный путь: создал проект → перешёл в
    // Settings → projectProvider сразу AsyncData без перехода loading→data),
    // listen не дёрнется и контроллеры останутся пустыми. Дублируем apply
    // здесь, защищая от лишних setState через _lastApplied.
    if (!_dirty &&
        !_suppressNextProviderListenApply &&
        (_lastApplied?.id != project.id ||
            _lastApplied?.updatedAt != project.updatedAt)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          setState(() => _applyFromProject(project));
        }
      });
    }

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
          child: DefaultTabController(
            length: 9,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TabBar(
                  isScrollable: true,
                  tabs: [
                    Tab(text: l10n.projectSettingsTabGeneral),
                    Tab(text: l10n.projectSettingsTabVariables),
                    Tab(text: AppLocalizations.of(context)!.webhooksTitle),
                    Tab(text: AppLocalizations.of(context)!.assistantPromptProjectTabTitle),
                    Tab(text: AppLocalizations.of(context)!.enhancerTabTitle),
                    Tab(text: AppLocalizations.of(context)!.scoutTabTitle),
                    Tab(text: AppLocalizations.of(context)!.sandboxServicesTabTitle),
                    Tab(text: AppLocalizations.of(context)!.assistantMcpTabTitle),
                    Tab(text: AppLocalizations.of(context)!.repoEnvFilesTabTitle),
                  ],
                ),
                Expanded(
                  child: TabBarView(
                    children: [
                      RefreshIndicator(
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
                                  branchTemplateController: _branchTemplateCtrl,
                                  branchPatternController: _branchPatternCtrl,
                                  mrTitleController: _mrTitleCtrl,
                                  branchNamingLocked: _branchNamingLocked,
                                  onBranchNamingLockedChanged: (v) {
                                    setState(() {
                                      _branchNamingLocked = v;
                                      _dirty = true;
                                    });
                                  },
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
                                ProjectGitAccountSection(project: project),
                                const SizedBox(height: 24),
                                ProjectSettingsVectorSection(
                                  vectorController: _vectorCtrl,
                                  project: project,
                                  reindexDisabled: reindexDisabled,
                                  onVectorChanged: _markDirty,
                                  onReindex: _onReindex,
                                ),
                                const SizedBox(height: 24),
                                ProjectRepositoriesSection(
                                  projectId: widget.projectId,
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
                      // Таб «Переменные (Технологический стек)»: tech stack (часть
                      // настроек проекта, сохраняется кнопкой ниже) + переменные/секреты
                      // проекта (сохраняются самостоятельно через ProjectVariablesSection).
                      RefreshIndicator(
                        onRefresh: onRefresh,
                        child: SingleChildScrollView(
                          physics: const AlwaysScrollableScrollPhysics(),
                          padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [
                              ProjectSettingsTechStackSection(
                                rows: _techRows,
                                onAddRow: _addTechRow,
                                onRemoveRow: _removeTechRow,
                                onClearTechStack: _onClearTechStack,
                                onRowChanged: _markDirty,
                              ),
                              const SizedBox(height: 16),
                              FilledButton(
                                onPressed: _saveBusy ? null : () => _onSave(project),
                                child: Text(l10n.projectSettingsSave),
                              ),
                              const SizedBox(height: 32),
                              const Divider(),
                              const SizedBox(height: 16),
                              ProjectVariablesSection(projectId: widget.projectId),
                            ],
                          ),
                        ),
                      ),
                      SingleChildScrollView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
                        child: WebhooksListSection(projectId: widget.projectId),
                      ),
                      SingleChildScrollView(
                        physics: const AlwaysScrollableScrollPhysics(),
                        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
                        child: AssistantPromptEditor(
                          // key по наличию снапшота: после сохранения/сброса
                          // editor пересоздаётся с актуальным initialValue.
                          key: ValueKey(
                            'assistant-prompt-${project.assistantPrompt != null}-'
                            '${project.assistantPrompt?.length ?? 0}',
                          ),
                          heading: l10n.assistantPromptProjectHeading,
                          hint: l10n.assistantPromptProjectHint,
                          initialValue: project.assistantPrompt ?? '',
                          inheritedNotice: project.assistantPrompt == null
                              ? l10n.assistantPromptInherited
                              : null,
                          onSave: (value) async {
                            await ref
                                .read(projectSettingsControllerProvider(
                                        widget.projectId)
                                    .notifier)
                                .save(UpdateProjectRequest(
                                    assistantPrompt: value));
                            ref.invalidate(projectProvider(widget.projectId));
                          },
                          onReset: project.assistantPrompt == null
                              ? null
                              : () async {
                                  await ref
                                      .read(projectSettingsControllerProvider(
                                              widget.projectId)
                                          .notifier)
                                      .save(const UpdateProjectRequest(
                                          assistantPrompt: ''));
                                  ref.invalidate(
                                      projectProvider(widget.projectId));
                                },
                        ),
                      ),
                      EnhancerSettingsTab(projectId: widget.projectId),
                      ScoutSettingsTab(projectId: widget.projectId),
                      SandboxServicesTab(projectId: widget.projectId),
                      AssistantMcpTab(projectId: widget.projectId),
                      RepoEnvFilesTab(projectId: widget.projectId),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}
