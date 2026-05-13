import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/safe_error_message.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:frontend/features/team/data/agent_settings_providers.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

/// Sprint 15.32 — диалог per-agent настроек code-backend.
///
/// Открывается из основного [showAgentEditDialog] кнопкой «Дополнительно».
/// 4 вкладки: «Модель/провайдер», «MCP-серверы», «Skills», «Разрешения Claude Code».
///
/// Возвращает true, если что-то было сохранено (вызывающий может перечитать данные).
Future<bool?> showAgentSandboxSettingsDialog(
  BuildContext context, {
  required String agentID,
}) {
  return showDialog<bool>(
    context: context,
    barrierDismissible: false,
    builder: (_) => _Dialog(agentID: agentID),
  );
}

class _Dialog extends ConsumerWidget {
  const _Dialog({required this.agentID});

  final String agentID;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n =
        requireAppLocalizations(context, where: 'agentSandboxSettingsDialog');
    final asyncSettings = ref.watch(agentSettingsProvider(agentID));
    // Sprint 15.Major5/N6: LayoutBuilder получает incoming constraints от Dialog'а
    // (родитель ограничивает viewport), и мы вычисляем maxW/maxH от них. Это работает
    // корректно для вложенных модалов / split-screen / desktop multi-window — там
    // MediaQuery.sizeOf(context) даёт размер всего экрана, а не контейнера.
    return Dialog(
      child: LayoutBuilder(
        builder: (context, incoming) {
          final maxH = (incoming.maxHeight * 0.95).clamp(360.0, 720.0);
          final maxW = (incoming.maxWidth * 0.98).clamp(320.0, 720.0);
          return ConstrainedBox(
            constraints: BoxConstraints(maxWidth: maxW, maxHeight: maxH),
            child: asyncSettings.when(
              loading: () => const Padding(
                padding: EdgeInsets.all(48),
                child: Center(child: CircularProgressIndicator()),
              ),
              error: (err, _) => Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(l10n.agentSandboxSettingsLoadError),
                    const SizedBox(height: 8),
                    SelectableText(safeErrorMessage(context, err)),
                    const SizedBox(height: 12),
                    OutlinedButton(
                      onPressed: () =>
                          ref.invalidate(agentSettingsProvider(agentID)),
                      child: Text(l10n.retry),
                    ),
                  ],
                ),
              ),
              data: (current) => _Body(agentID: agentID, current: current),
            ),
          );
        },
      ),
    );
  }
}

class _Body extends ConsumerStatefulWidget {
  const _Body({required this.agentID, required this.current});
  final String agentID;
  final AgentSettingsModel current;

  @override
  ConsumerState<_Body> createState() => _BodyState();
}

class _BodyState extends ConsumerState<_Body>
    with SingleTickerProviderStateMixin {
  late final TabController _tabs;
  late String? _llmProviderID = widget.current.llmProviderID;
  late String? _codeBackend = widget.current.codeBackend;
  late final TextEditingController _mcpJSON = TextEditingController(
    text: const JsonEncoder.withIndent('  ').convert(
      widget.current.codeBackendSettings['mcp_servers'] ?? <dynamic>[],
    ),
  );
  late final TextEditingController _skillsJSON = TextEditingController(
    text: const JsonEncoder.withIndent('  ').convert(
      widget.current.codeBackendSettings['skills'] ?? <dynamic>[],
    ),
  );
  late final _PermissionsForm _permissionsForm =
      _PermissionsForm.fromMap(widget.current.sandboxPermissions);
  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _tabs = TabController(length: 4, vsync: this);
  }

  @override
  void dispose() {
    _tabs.dispose();
    _mcpJSON.dispose();
    _skillsJSON.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    final l10n =
        requireAppLocalizations(context, where: 'agentSandboxSettingsDialog');
    setState(() {
      _busy = true;
      _error = null;
    });

    Map<String, dynamic> codeBackendSettings;
    try {
      codeBackendSettings = {
        ...widget.current.codeBackendSettings,
        'mcp_servers': jsonDecode(_mcpJSON.text.trim().isEmpty
            ? '[]'
            : _mcpJSON.text),
        'skills': jsonDecode(_skillsJSON.text.trim().isEmpty
            ? '[]'
            : _skillsJSON.text),
      };
    } catch (e) {
      setState(() {
        _busy = false;
        _error = '${l10n.agentSandboxSettingsJsonInvalid}: $e';
      });
      return;
    }

    final repo = ref.read(agentSettingsRepositoryProvider);
    try {
      await repo.update(
        widget.agentID,
        llmProviderID: _llmProviderID,
        clearLLMProvider: _llmProviderID == null,
        codeBackend: _codeBackend,
        codeBackendSettings: codeBackendSettings,
        sandboxPermissions: _permissionsForm.toMap(),
      );
      ref.invalidate(agentSettingsProvider(widget.agentID));
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (err) {
      // Sprint 15.B (F10, C1): sanitized message через общий хелпер (FeatureException/ApiException).
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = safeErrorMessage(context, err);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'agentSandboxSettingsDialog');
    // Sprint 15.N6: Dialog в _Dialog задаёт maxHeight ConstrainedBox'ом; Column тут разворачивается
    // на всю доступную высоту, header/tabbar/actions — фиксированные строки, TabBarView —
    // Expanded. Никакой математики над MediaQuery — flex layout справится сам.
    return Column(
      mainAxisSize: MainAxisSize.max,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 8, 0),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  l10n.agentSandboxSettingsTitle,
                  style: Theme.of(context).textTheme.titleLarge,
                ),
              ),
              IconButton(
                tooltip: l10n.cancel,
                onPressed:
                    _busy ? null : () => Navigator.of(context).pop(false),
                icon: const Icon(Icons.close),
              ),
            ],
          ),
        ),
        TabBar(
          controller: _tabs,
          isScrollable: true,
          tabs: [
            Tab(text: l10n.agentSandboxSettingsTabProvider),
            Tab(text: l10n.agentSandboxSettingsTabMCP),
            Tab(text: l10n.agentSandboxSettingsTabSkills),
            Tab(text: l10n.agentSandboxSettingsTabPermissions),
          ],
        ),
        Expanded(
          child: TabBarView(
            controller: _tabs,
            children: [
              _ProviderTab(
                value: _llmProviderID,
                onChanged: (v) => setState(() => _llmProviderID = v),
                codeBackend: _codeBackend,
                onCodeBackendChanged: (v) => setState(() => _codeBackend = v),
              ),
              _JsonTab(
                controller: _mcpJSON,
                helperText: l10n.agentSandboxSettingsMCPHelper,
              ),
              _JsonTab(
                controller: _skillsJSON,
                helperText: l10n.agentSandboxSettingsSkillsHelper,
              ),
              _PermissionsTab(form: _permissionsForm),
            ],
          ),
        ),
        if (_error != null)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: SelectableText(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.end,
            children: [
              TextButton(
                onPressed: _busy ? null : () => Navigator.of(context).pop(false),
                child: Text(l10n.cancel),
              ),
              const SizedBox(width: 8),
              FilledButton(
                onPressed: _busy ? null : _save,
                child: Text(_busy ? '…' : l10n.save),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// Вкладка 1 — модель/провайдер.
class _ProviderTab extends ConsumerWidget {
  const _ProviderTab({
    required this.value,
    required this.onChanged,
    required this.codeBackend,
    required this.onCodeBackendChanged,
  });

  final String? value;
  final ValueChanged<String?> onChanged;
  final String? codeBackend;
  final ValueChanged<String?> onCodeBackendChanged;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n =
        requireAppLocalizations(context, where: 'agentSandboxSettingsDialog');
    final asyncProviders = ref.watch(llmProvidersListProvider);
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          asyncProviders.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => SelectableText(safeErrorMessage(context, err)),
            data: (list) => DropdownButtonFormField<String?>(
              initialValue: value,
              decoration: InputDecoration(
                labelText: l10n.agentSandboxSettingsProviderLabel,
              ),
              items: [
                DropdownMenuItem<String?>(
                  value: null,
                  child: Text(l10n.agentSandboxSettingsProviderNone),
                ),
                ...list.map((LLMProviderModel p) => DropdownMenuItem<String?>(
                      value: p.id,
                      child: Text('${p.name}  (${p.kind})'),
                    )),
              ],
              onChanged: onChanged,
            ),
          ),
          const SizedBox(height: 16),
          DropdownButtonFormField<String?>(
            initialValue: codeBackend,
            decoration: InputDecoration(
              labelText: l10n.agentSandboxSettingsCodeBackendLabel,
            ),
            items: [
              DropdownMenuItem<String?>(
                value: null,
                child: Text(l10n.agentSandboxSettingsProviderNone),
              ),
              ...kSupportedCodeBackends.map((b) => DropdownMenuItem<String?>(
                    value: b,
                    child: Text(b),
                  )),
            ],
            onChanged: onCodeBackendChanged,
          ),
        ],
      ),
    );
  }
}

/// Универсальная JSON-вкладка для MCP-серверов и Skills.
class _JsonTab extends StatelessWidget {
  const _JsonTab({required this.controller, required this.helperText});

  final TextEditingController controller;
  final String helperText;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: TextField(
        controller: controller,
        maxLines: null,
        expands: true,
        textAlignVertical: TextAlignVertical.top,
        decoration: InputDecoration(
          alignLabelWithHint: true,
          helperText: helperText,
          border: const OutlineInputBorder(),
        ),
      ),
    );
  }
}

/// Состояние формы permissions для вкладки 4.
class _PermissionsForm {
  _PermissionsForm({
    required this.allow,
    required this.deny,
    required this.ask,
    required this.defaultMode,
    required this.original,
  });

  /// Sprint 15.F-M5: запоминаем оригинальный объект sandbox_permissions,
  /// чтобы toMap() мог merge'ить неизвестные UI ключи (hooks, env, future fields).
  factory _PermissionsForm.fromMap(Map<String, dynamic> m) {
    List<String> read(String key) =>
        ((m[key] as List<dynamic>?) ?? const <dynamic>[])
            .map((e) => e.toString())
            .toList();
    return _PermissionsForm(
      allow: read('allow'),
      deny: read('deny'),
      ask: read('ask'),
      defaultMode: (m['defaultMode'] as String?) ?? '',
      original: Map<String, dynamic>.from(m),
    );
  }

  final List<String> allow;
  final List<String> deny;
  final List<String> ask;
  String defaultMode;
  final Map<String, dynamic> original;

  /// toMap() возвращает merge неизвестных ключей оригинала + редактируемые поля формы.
  /// Sprint 15.F-M5: не затирает hooks/env/etc., которые ввели через прямой API/MCP.
  Map<String, dynamic> toMap() => {
        ...original,
        'allow': allow,
        'deny': deny,
        'ask': ask,
        'defaultMode': defaultMode,
      };
}

class _PermissionsTab extends StatefulWidget {
  const _PermissionsTab({required this.form});
  final _PermissionsForm form;

  @override
  State<_PermissionsTab> createState() => _PermissionsTabState();
}

class _PermissionsTabState extends State<_PermissionsTab> {
  @override
  Widget build(BuildContext context) {
    final l10n =
        requireAppLocalizations(context, where: 'agentSandboxSettingsDialog');
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          DropdownButtonFormField<String>(
            initialValue:
                widget.form.defaultMode.isEmpty ? 'default' : widget.form.defaultMode,
            decoration: InputDecoration(
              labelText: l10n.agentSandboxSettingsDefaultMode,
            ),
            items: kSupportedPermissionModes
                .map((m) => DropdownMenuItem(value: m, child: Text(m)))
                .toList(),
            onChanged: (v) => setState(() => widget.form.defaultMode = v ?? ''),
          ),
          const SizedBox(height: 16),
          _PatternListField(
            label: l10n.agentSandboxSettingsAllow,
            values: widget.form.allow,
            onChange: () => setState(() {}),
          ),
          _PatternListField(
            label: l10n.agentSandboxSettingsDeny,
            values: widget.form.deny,
            onChange: () => setState(() {}),
          ),
          _PatternListField(
            label: l10n.agentSandboxSettingsAsk,
            values: widget.form.ask,
            onChange: () => setState(() {}),
          ),
        ],
      ),
    );
  }
}

class _PatternListField extends StatefulWidget {
  const _PatternListField({
    required this.label,
    required this.values,
    required this.onChange,
  });

  final String label;
  final List<String> values;
  final VoidCallback onChange;

  @override
  State<_PatternListField> createState() => _PatternListFieldState();
}

class _PatternListFieldState extends State<_PatternListField> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _add() {
    final v = _controller.text.trim();
    if (v.isEmpty) return;
    setState(() {
      widget.values.add(v);
      _controller.clear();
    });
    widget.onChange();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(widget.label, style: Theme.of(context).textTheme.titleSmall),
          const SizedBox(height: 4),
          Wrap(
            spacing: 6,
            runSpacing: 4,
            children: [
              for (final v in List<String>.from(widget.values))
                InputChip(
                  label: Text(v),
                  onDeleted: () {
                    setState(() => widget.values.remove(v));
                    widget.onChange();
                  },
                ),
            ],
          ),
          Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _controller,
                  onSubmitted: (_) => _add(),
                  decoration: InputDecoration(
                    hintText: requireAppLocalizations(context,
                            where: 'agentSandboxSettingsDialog.patternHint')
                        .agentSandboxSettingsPatternHint,
                  ),
                ),
              ),
              IconButton(icon: const Icon(Icons.add), onPressed: _add),
            ],
          ),
        ],
      ),
    );
  }
}
