import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/json/patch.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/admin/prompts/data/prompts_providers.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_exceptions.dart';
import 'package:frontend/features/admin/prompts/domain/prompt_model.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart'
    show AgentModel, codeBackends;
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/data/tools_providers.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart'
    show kSupportedAgentProviderKinds;
import 'package:frontend/features/team/domain/models/tool_definition_model.dart';
import 'package:frontend/features/team/domain/team_exceptions.dart';
import 'package:frontend/features/team/domain/tool_binding_patch_item.dart';
import 'package:frontend/features/team/domain/tools_exceptions.dart';
import 'package:frontend/features/team/domain/update_agent_patch.dart';
import 'package:frontend/features/team/presentation/widgets/agent_sandbox_settings_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Только для `test/` — в продакшене используйте [showAgentEditDialog].
@visibleForTesting
Widget agentEditDialogBodyForTesting({
  required String projectId,
  required AgentModel agent,
  required bool useAutofocus,
}) =>
    _AgentEditDialogBody(
      projectId: projectId,
      agent: agent,
      useAutofocus: useAutofocus,
    );

/// Диалог / bottom sheet: редактирование агента (13.3).
Future<void> showAgentEditDialog(
  BuildContext context, {
  required String projectId,
  required AgentModel agent,
}) async {
  assert(projectId.isNotEmpty);
  assert(agent.id.isNotEmpty);
  final width = MediaQuery.sizeOf(context).width;
  final wide = width >= 600;
  if (wide) {
    await showDialog<void>(
      context: context,
      // Закрытие тапом по барьеру недоступно: выход через «Отмена» / после сохранения.
      barrierDismissible: false,
      builder: (ctx) {
        return Dialog(
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 560),
            child: _AgentEditDialogBody(
              projectId: projectId,
              agent: agent,
              useAutofocus: true,
            ),
          ),
        );
      },
    );
  } else {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      isDismissible: false,
      enableDrag: false,
      builder: (ctx) {
        return Padding(
          padding: EdgeInsets.only(
            bottom: MediaQuery.viewInsetsOf(ctx).bottom,
          ),
          child: _AgentEditDialogBody(
            projectId: projectId,
            agent: agent,
            useAutofocus: false,
          ),
        );
      },
    );
  }
}

class _AgentEditDialogBody extends ConsumerStatefulWidget {
  const _AgentEditDialogBody({
    required this.projectId,
    required this.agent,
    required this.useAutofocus,
  });

  final String projectId;
  final AgentModel agent;
  final bool useAutofocus;

  @override
  ConsumerState<_AgentEditDialogBody> createState() =>
      _AgentEditDialogBodyState();
}

class _AgentEditDialogBodyState extends ConsumerState<_AgentEditDialogBody> {
  static const _modelMaxLen = 128;

  late final TextEditingController _modelController;
  late final FocusNode _modelFocus;
  late final AgentModel _initial;

  CancelToken? _promptsCancel;
  CancelToken? _patchCancel;
  CancelToken? _toolsCancel;
  bool _promptsLoading = true;
  Object? _promptsError;
  List<Prompt> _prompts = [];

  bool _toolsLoading = true;
  Object? _toolsError;
  List<ToolDefinitionModel> _toolDefinitions = [];

  late final Set<String> _initialToolBindingIds;
  final Set<String> _selectedToolDefIds = {};

  String? _promptId;
  bool _promptTouched = false;

  String? _codeBackend;
  String? _providerKind;
  late bool _isActive;

  bool _dirty = false;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _initial = widget.agent;
    _modelController = TextEditingController(text: widget.agent.model ?? '');
    _modelController.addListener(_recomputeDirty);
    _modelFocus = FocusNode();
    _promptId = widget.agent.promptId;
    _codeBackend = widget.agent.codeBackend;
    _providerKind = widget.agent.providerKind;
    _isActive = widget.agent.isActive;
    _loadPrompts();
    _initialToolBindingIds = widget.agent.toolBindings
        .map((b) => b.toolDefinitionId)
        .toSet();
    _selectedToolDefIds.addAll(_initialToolBindingIds);
    _loadToolDefinitions();
  }

  // Ревью: _loadPrompts и _loadToolDefinitions структурно похожи (DRY) — выносить общий шаблон
  // только при появлении третьей boot-секции (например MCP из миграции 016), YAGNI.
  //
  // Повторный параллельный вызов loader из виджета сейчас не используется (нет didUpdateWidget);
  // отмена предыдущего CancelToken в начале каждого _load* покрывает refresh/retry.
  Future<void> _loadPrompts() async {
    _promptsCancel?.cancel();
    _promptsCancel = CancelToken();
    final token = _promptsCancel;
    setState(() {
      _promptsLoading = true;
      _promptsError = null;
    });
    try {
      final list = await ref.read(promptsRepositoryProvider).getPrompts(
            cancelToken: token,
          );
      if (!mounted) {
        return;
      }
      setState(() {
        _prompts = list;
        _promptsLoading = false;
        final pid = _promptId;
        if (pid != null &&
            pid.isNotEmpty &&
            !list.any((p) => p.id == pid)) {
          _promptTouched = true;
          _promptId = null;
        }
      });
      _recomputeDirty();
    } catch (e) {
      if (!mounted) {
        return;
      }
      if (e is PromptCancelledException) {
        return;
      }
      setState(() {
        _promptsError = e;
        _promptsLoading = false;
      });
    }
  }

  Future<void> _loadToolDefinitions() async {
    _toolsCancel?.cancel();
    _toolsCancel = CancelToken();
    final token = _toolsCancel;
    setState(() {
      _toolsLoading = true;
      _toolsError = null;
    });
    try {
      final list = await ref.read(toolsRepositoryProvider).fetchToolDefinitions(
            cancelToken: token,
          );
      if (!mounted) {
        return;
      }
      setState(() {
        _toolDefinitions = list;
        _toolsLoading = false;
      });
      _recomputeDirty();
    } catch (e) {
      if (!mounted) {
        return;
      }
      if (e is ToolsCancelledException) {
        return;
      }
      setState(() {
        _toolsError = e;
        _toolsLoading = false;
      });
    }
  }

  @override
  void dispose() {
    _promptsCancel?.cancel();
    _patchCancel?.cancel();
    _toolsCancel?.cancel();
    _modelController.removeListener(_recomputeDirty);
    _modelController.dispose();
    _modelFocus.dispose();
    super.dispose();
  }

  void _recomputeDirty() {
    final modelTrim = _modelController.text.trim();
    final initialModel = (_initial.model ?? '').trim();
    final modelDirty = modelTrim != initialModel;

    final promptDirty = _promptTouched &&
        (_promptId ?? '') != (_initial.promptId ?? '');

    final cbDirty = (_codeBackend ?? '') != (_initial.codeBackend ?? '');
    final pkDirty = (_providerKind ?? '') != (_initial.providerKind ?? '');
    final activeDirty = _isActive != _initial.isActive;

    final toolsDirty = !_sameToolIdSet(_selectedToolDefIds, _initialToolBindingIds);

    final next =
        modelDirty || promptDirty || cbDirty || pkDirty || activeDirty || toolsDirty;
    if (next != _dirty) {
      setState(() => _dirty = next);
    }
  }

  bool _sameToolIdSet(Set<String> a, Set<String> b) {
    if (a.length != b.length) {
      return false;
    }
    for (final id in a) {
      if (!b.contains(id)) {
        return false;
      }
    }
    return true;
  }

  Future<bool> _confirmDiscard() async {
    final l10n = requireAppLocalizations(
      context,
      where: 'agentEditDialog.confirmDiscard',
    );
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.teamAgentEditDiscardTitle),
        content: Text(l10n.teamAgentEditDiscardBody),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.teamAgentEditCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.teamAgentEditDiscardConfirm),
          ),
        ],
      ),
    );
    return ok == true;
  }

  Future<void> _onCancel() async {
    if (_saving) {
      return;
    }
    if (_dirty) {
      final ok = await _confirmDiscard();
      if (!ok || !mounted) {
        return;
      }
    }
    if (mounted) {
      Navigator.of(context).pop();
    }
  }

  UpdateAgentPatch _buildPatch() {
    final modelTrim = _modelController.text.trim();
    final initialModel = (_initial.model ?? '').trim();
    var modelPatch = const Patch<String>.omit();
    if (modelTrim != initialModel) {
      modelPatch = modelTrim.isEmpty
          ? const Patch<String>.clear()
          : Patch.value(modelTrim);
    }

    final Patch<String?> promptPatch;
    if (_promptTouched) {
      final cur = _promptId;
      final ini = _initial.promptId;
      if ((cur ?? '') != (ini ?? '')) {
        promptPatch =
            cur == null || cur.isEmpty
                ? const Patch<String?>.clear()
                : Patch.value(cur);
      } else {
        promptPatch = const Patch<String?>.omit();
      }
    } else {
      promptPatch = const Patch<String?>.omit();
    }

    final Patch<String?> cbPatch;
    if ((_codeBackend ?? '') != (_initial.codeBackend ?? '')) {
      cbPatch = _codeBackend == null || _codeBackend!.isEmpty
          ? const Patch<String?>.clear()
          : Patch.value(_codeBackend!);
    } else {
      cbPatch = const Patch<String?>.omit();
    }

    final Patch<String?> pkPatch;
    if ((_providerKind ?? '') != (_initial.providerKind ?? '')) {
      pkPatch = _providerKind == null || _providerKind!.isEmpty
          ? const Patch<String?>.clear()
          : Patch.value(_providerKind!);
    } else {
      pkPatch = const Patch<String?>.omit();
    }

    final Patch<bool> activePatch;
    if (_isActive != _initial.isActive) {
      activePatch = Patch.value(_isActive);
    } else {
      activePatch = const Patch<bool>.omit();
    }

    final Patch<List<ToolBindingPatchItem>> toolsPatch;
    if (!_sameToolIdSet(_selectedToolDefIds, _initialToolBindingIds)) {
      final sorted = _selectedToolDefIds.toList()..sort();
      toolsPatch = Patch.value(
        sorted
            .map((id) => ToolBindingPatchItem(toolDefinitionId: id))
            .toList(),
      );
    } else {
      toolsPatch = const Patch<List<ToolBindingPatchItem>>.omit();
    }

    return UpdateAgentPatch(
      model: modelPatch,
      promptId: promptPatch,
      codeBackend: cbPatch,
      providerKind: pkPatch,
      isActive: activePatch,
      toolBindings: toolsPatch,
    );
  }

  Future<void> _save() async {
    if (_saving) {
      return;
    }
    final l10n = requireAppLocalizations(context, where: 'agentEditDialog.save');
    final patch = _buildPatch();
    final body = patch.toWireJson();
    if (body.isEmpty) {
      if (mounted) {
        Navigator.of(context).pop();
      }
      return;
    }

    final patchToken = CancelToken();
    _patchCancel = patchToken;

    setState(() => _saving = true);
    try {
      await ref.read(teamRepositoryProvider).patchAgent(
            widget.projectId,
            widget.agent.id,
            body,
            cancelToken: patchToken,
          );
    } on TeamApiException catch (e) {
      if (mounted) {
        setState(() => _saving = false);
        final sentTools = patch.toWireJson().containsKey('tool_bindings');
        if (e.statusCode == 400 && sentTools) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(l10n.teamAgentEditToolsValidationError)),
          );
        } else {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text(l10n.teamAgentEditSaveError)),
          );
        }
      }
      return;
    } on TeamConflictException {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.teamAgentEditConflictError)),
        );
      }
      return;
    } on TeamForbiddenException {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.teamAgentEditSaveForbidden)),
        );
      }
      return;
    } on TeamCancelledException {
      if (mounted) {
        setState(() => _saving = false);
      }
      return;
    } on TeamRepositoryException {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.teamAgentEditSaveError)),
        );
      }
      return;
    } catch (_) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.teamAgentEditSaveError)),
        );
      }
      return;
    }

    if (!mounted) {
      return;
    }
    ref.invalidate(teamProvider(widget.projectId));
    final messenger = ScaffoldMessenger.of(context);
    try {
      await ref.read(teamProvider(widget.projectId).future);
    } on Exception {
      if (!mounted) {
        return;
      }
      // UX: при ошибке refetch после успешного PATCH закрываем форму и показываем SnackBar на
      // родительском ScaffoldMessenger. Если продукт решит оставлять диалог открытым — менять здесь.
      Navigator.of(context).pop();
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.teamAgentEditRefetchError)),
      );
      return;
    }
    if (!mounted) {
      return;
    }
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agentEditDialog.body');
    final theme = Theme.of(context);

    return PopScope(
      canPop: !_saving && !_dirty,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop || !mounted) {
          return;
        }
        if (_saving) {
          return;
        }
        final ok = await _confirmDiscard();
        if (ok && context.mounted) {
          Navigator.of(context).pop();
        }
      },
      // При _saving: IgnorePointer отключает ввод под прокруткой; AbsorbPointer на оверлее —
      // визуальный «занавес» и перехват тачей поверх (дублирование намеренно).
      child: Stack(
        children: [
          IgnorePointer(
            ignoring: _saving,
            child: SingleChildScrollView(
              key: const Key('agentEditDialogBody'),
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                Text(
                  l10n.teamAgentEditTitle,
                  style: theme.textTheme.titleLarge,
                ),
                const SizedBox(height: 16),
                _ReadonlyAgentSummary(agent: widget.agent, l10n: l10n),
                const SizedBox(height: 16),
                TextFormField(
                  controller: _modelController,
                  focusNode: _modelFocus,
                  autofocus: widget.useAutofocus,
                  decoration: InputDecoration(
                    labelText: l10n.teamAgentEditFieldModel,
                    border: const OutlineInputBorder(),
                  ),
                  maxLength: _modelMaxLen,
                  textInputAction: TextInputAction.next,
                  onChanged: (_) => _recomputeDirty(),
                ),
                const SizedBox(height: 12),
                _PromptField(
                  l10n: l10n,
                  loading: _promptsLoading,
                  error: _promptsError,
                  prompts: _prompts,
                  value: _promptId,
                  onChanged: (v) {
                    setState(() {
                      _promptId = v;
                      _promptTouched = true;
                    });
                    _recomputeDirty();
                  },
                ),
                const SizedBox(height: 12),
                _CodeBackendField(
                  l10n: l10n,
                  value: _codeBackend,
                  onChanged: (v) {
                    setState(() => _codeBackend = v);
                    _recomputeDirty();
                  },
                ),
                const SizedBox(height: 12),
                _ProviderKindField(
                  l10n: l10n,
                  value: _providerKind,
                  onChanged: (v) {
                    setState(() => _providerKind = v);
                    _recomputeDirty();
                  },
                ),
                const SizedBox(height: 8),
                SwitchListTile(
                  title: Text(l10n.teamAgentEditFieldActive),
                  value: _isActive,
                  onChanged: (v) {
                    setState(() => _isActive = v);
                    _recomputeDirty();
                  },
                ),
                const SizedBox(height: 16),
                _buildToolsSection(l10n, theme),
                const SizedBox(height: 24),
                // Sprint 15.N6: переключаем фиксированный Row на Wrap — кнопки переносятся
                // на 2 ряда при недостатке ширины (на портретном телефоне overflow=29px).
                Wrap(
                  alignment: WrapAlignment.spaceBetween,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    Wrap(
                      spacing: 8,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        TextButton(
                          onPressed: _saving ? null : _onCancel,
                          child: Text(l10n.teamAgentEditCancel),
                        ),
                        // Sprint 15.32 — advanced-настройки (модель/MCP/Skills/permissions).
                        OutlinedButton.icon(
                          icon: const Icon(Icons.tune),
                          onPressed: _saving
                              ? null
                              : () => showAgentSandboxSettingsDialog(
                                    context,
                                    agentID: widget.agent.id,
                                  ),
                          label: Text(l10n.teamAgentEditAdvanced),
                        ),
                      ],
                    ),
                    Wrap(
                      spacing: 8,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        if (_saving)
                          const SizedBox(
                            width: 24,
                            height: 24,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          ),
                        FilledButton(
                          onPressed: _saving ? null : _save,
                          child: Text(l10n.teamAgentEditSave),
                        ),
                      ],
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
        if (_saving)
          Positioned.fill(
              child: AbsorbPointer(
                child: DecoratedBox(
                  decoration: BoxDecoration(
                    color: theme.colorScheme.surface.withValues(alpha: 0.45),
                  ),
                  child: const Center(
                    child: SizedBox(
                      width: 40,
                      height: 40,
                      child: CircularProgressIndicator(),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildToolsSection(AppLocalizations l10n, ThemeData theme) {
    return KeyedSubtree(
      key: const Key('agentEditToolsSection'),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(
                child: Text(
                  l10n.teamAgentEditFieldTools,
                  style: theme.textTheme.titleSmall,
                ),
              ),
              if (!_toolsLoading)
                IconButton(
                  key: const Key('agentEditToolsRefreshCatalog'),
                  tooltip: l10n.teamAgentEditToolsRetry,
                  onPressed: _saving ? null : () => _loadToolDefinitions(),
                  icon: const Icon(Icons.refresh_outlined),
                ),
            ],
          ),
          const SizedBox(height: 8),
          if (_toolsLoading)
            const SizedBox(
              height: 56,
              child: Align(
                alignment: Alignment.centerLeft,
                child: SizedBox(
                  width: 22,
                  height: 22,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            )
          else if (_toolsError != null)
            InputDecorator(
              decoration: InputDecoration(
                labelText: l10n.teamAgentEditFieldTools,
                border: const OutlineInputBorder(),
                errorText: l10n.teamAgentEditToolsLoadError,
              ),
              child: Align(
                alignment: Alignment.centerLeft,
                child: TextButton(
                  onPressed: _saving ? null : () => _loadToolDefinitions(),
                  child: Text(l10n.teamAgentEditToolsRetry),
                ),
              ),
            )
          else if (_toolDefinitions.isEmpty)
            Text(
              l10n.teamAgentEditToolsEmpty,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            )
          else ...[
            if (_selectedToolDefIds.isEmpty)
              Padding(
                padding: const EdgeInsets.only(bottom: 8),
                child: Text(
                  l10n.teamAgentEditToolsNoneSelected,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final t in _toolDefinitions)
                  FilterChip(
                    label: Text(
                      l10n.teamAgentEditToolsListEntryLabel(t.name, t.category),
                    ),
                    selected: _selectedToolDefIds.contains(t.id),
                    onSelected: _saving
                        ? null
                        : (sel) {
                            setState(() {
                              if (sel) {
                                _selectedToolDefIds.add(t.id);
                              } else {
                                _selectedToolDefIds.remove(t.id);
                              }
                            });
                            _recomputeDirty();
                          },
                  ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _ReadonlyAgentSummary extends StatelessWidget {
  const _ReadonlyAgentSummary({
    required this.agent,
    required this.l10n,
  });

  final AgentModel agent;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final name = agent.name.trim().isEmpty ? l10n.teamAgentNameUnset : agent.name;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(name, style: theme.textTheme.titleMedium),
        const SizedBox(height: 4),
        Text(
          agentRoleLabel(l10n, agent.role),
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _PromptField extends StatelessWidget {
  const _PromptField({
    required this.l10n,
    required this.loading,
    required this.error,
    required this.prompts,
    required this.value,
    required this.onChanged,
  });

  final AppLocalizations l10n;
  final bool loading;
  final Object? error;
  final List<Prompt> prompts;
  final String? value;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: l10n.teamAgentEditFieldPrompt,
          border: const OutlineInputBorder(),
        ),
        child: const SizedBox(
          height: 24,
          child: Align(
            alignment: Alignment.centerLeft,
            child: SizedBox(
              width: 22,
              height: 22,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        ),
      );
    }
    if (error != null) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: l10n.teamAgentEditFieldPrompt,
          border: const OutlineInputBorder(),
          errorText: l10n.teamAgentEditPromptsLoadError,
        ),
        child: const SizedBox.shrink(),
      );
    }
    if (prompts.isEmpty) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: l10n.teamAgentEditFieldPrompt,
          border: const OutlineInputBorder(),
        ),
        child: Text(
          l10n.teamAgentEditNoPrompts,
          style: Theme.of(context).textTheme.bodyMedium,
        ),
      );
    }

    final items = <DropdownMenuItem<String?>>[
      DropdownMenuItem<String?>(
        value: null,
        child: Text(l10n.teamAgentEditPromptNone),
      ),
      ...prompts.map(
        (p) => DropdownMenuItem<String?>(
          value: p.id,
          child: Text(p.name),
        ),
      ),
    ];

    // initialValue — не «value из родителя на каждый build»: при программном сбросе/rollback
    // _promptId без пользовательского onChanged пересоздайте поле (Key), иначе FormField не обновит UI.
    return DropdownButtonFormField<String?>(
      decoration: InputDecoration(
        labelText: l10n.teamAgentEditFieldPrompt,
        border: const OutlineInputBorder(),
      ),
      isExpanded: true,
      initialValue: _effectiveValue(value, prompts),
      items: items,
      onChanged: onChanged,
    );
  }

  /// Страховка: [current] отсутствует в [list] — показываем «не выбран», чтобы не падать на assert
  /// у [DropdownButtonFormField] (основная нормализация — в State формы при загрузке промптов).
  String? _effectiveValue(String? current, List<Prompt> list) {
    if (current == null || current.isEmpty) {
      return null;
    }
    final ok = list.any((p) => p.id == current);
    return ok ? current : null;
  }
}

class _CodeBackendField extends StatelessWidget {
  const _CodeBackendField({
    required this.l10n,
    required this.value,
    required this.onChanged,
  });

  final AppLocalizations l10n;
  final String? value;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    final items = <DropdownMenuItem<String?>>[
      DropdownMenuItem<String?>(
        value: null,
        child: Text(l10n.teamAgentEditUnset),
      ),
      ...codeBackends.map(
        (b) => DropdownMenuItem<String?>(
          value: b,
          child: Text(b),
        ),
      ),
    ];

    // См. _PromptField: initialValue; программный сброс _codeBackend без onChanged — Key на поле.
    return DropdownButtonFormField<String?>(
      decoration: InputDecoration(
        labelText: l10n.teamAgentEditFieldCodeBackend,
        border: const OutlineInputBorder(),
      ),
      isExpanded: true,
      initialValue: value != null && codeBackends.contains(value) ? value : null,
      items: items,
      onChanged: onChanged,
    );
  }
}

/// Sprint 15.e2e — dropdown для `agent.provider_kind`. Резолвер на бэке по этому
/// kind берёт ключ из user_llm_credentials (или OAuth-подписки) и выставляет
/// нужные env в sandbox-контейнере.
class _ProviderKindField extends StatelessWidget {
  const _ProviderKindField({
    required this.l10n,
    required this.value,
    required this.onChanged,
  });

  final AppLocalizations l10n;
  final String? value;
  final ValueChanged<String?> onChanged;

  @override
  Widget build(BuildContext context) {
    final items = <DropdownMenuItem<String?>>[
      DropdownMenuItem<String?>(
        value: null,
        child: Text(l10n.teamAgentEditUnset),
      ),
      ...kSupportedAgentProviderKinds.map(
        (k) => DropdownMenuItem<String?>(
          value: k,
          child: Text(k),
        ),
      ),
    ];
    return DropdownButtonFormField<String?>(
      key: const Key('agentEditDialog_providerKindField'),
      decoration: InputDecoration(
        labelText: l10n.teamAgentEditFieldProviderKind,
        helperText: l10n.teamAgentEditFieldProviderKindHelp,
        border: const OutlineInputBorder(),
      ),
      isExpanded: true,
      initialValue:
          value != null && kSupportedAgentProviderKinds.contains(value)
              ? value
              : null,
      items: items,
      onChanged: onChanged,
    );
  }
}
