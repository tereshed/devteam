import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/scout/domain/models/scout_config_model.dart';
import 'package:frontend/features/team/domain/agent_provider_rules.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

/// Колбэк сохранения конфига разведчика (частичный апдейт; промпт — отдельно).
/// Зеркалит агентную настройку: бэкенд + провайдер + модель + temperature +
/// code_backend_settings (mcp/скиллы) + sandbox_permissions.
typedef ScoutConfigSave = Future<void> Function({
  bool? isEnabled,
  String? codeBackend,
  String? providerKind,
  double? temperature,
  Map<String, dynamic>? codeBackendSettings,
  Map<String, dynamic>? sandboxPermissions,
  int? timeoutSeconds,
});

/// Форма конфига разведчика — как настройка агента, но всегда sandbox.
class ScoutConfigForm extends StatefulWidget {
  const ScoutConfigForm({
    super.key,
    required this.config,
    required this.onSave,
  });

  final ScoutConfigModel config;
  final ScoutConfigSave onSave;

  @override
  State<ScoutConfigForm> createState() => _ScoutConfigFormState();
}

class _ScoutConfigFormState extends State<ScoutConfigForm> {
  late bool _isEnabled;
  late String _codeBackend;
  String? _providerKind;
  late double _temperature;
  late final TextEditingController _modelCtrl;
  late final TextEditingController _timeoutCtrl;
  late final TextEditingController _mcpCtrl;
  late final TextEditingController _skillsCtrl;
  late final TextEditingController _permsCtrl;
  bool _busy = false;
  bool _dirty = false;

  @override
  void initState() {
    super.initState();
    final c = widget.config;
    _isEnabled = c.isEnabled;
    _codeBackend = kSupportedCodeBackends.contains(c.codeBackend)
        ? c.codeBackend
        : kSupportedCodeBackends.first;
    _providerKind = c.providerKind;
    _temperature = c.temperature ?? 0.7;
    _modelCtrl = TextEditingController(
      text: (c.codeBackendSettings['model'] as String?) ?? '',
    );
    _timeoutCtrl = TextEditingController(text: c.timeoutSeconds.toString());
    _mcpCtrl = TextEditingController(text: _pretty(c.codeBackendSettings['mcp_servers']));
    _skillsCtrl = TextEditingController(text: _pretty(c.codeBackendSettings['skills']));
    _permsCtrl = TextEditingController(
      text: c.sandboxPermissions.isEmpty ? '' : _pretty(c.sandboxPermissions),
    );
  }

  @override
  void dispose() {
    _modelCtrl.dispose();
    _timeoutCtrl.dispose();
    _mcpCtrl.dispose();
    _skillsCtrl.dispose();
    _permsCtrl.dispose();
    super.dispose();
  }

  static String _pretty(dynamic value) {
    if (value == null) {
      return '';
    }
    return const JsonEncoder.withIndent('  ').convert(value);
  }

  void _markDirty() {
    if (!_dirty) {
      setState(() => _dirty = true);
    }
  }

  void _onBackendChanged(String? v) {
    if (v == null) {
      return;
    }
    setState(() {
      _codeBackend = v;
      // Сбросить провайдера, если он больше не разрешён для бэкенда.
      final allowed = allowedProviderKindsForBackend(v);
      if (_providerKind != null && !allowed.contains(_providerKind)) {
        _providerKind = null;
      }
    });
    _markDirty();
  }

  /// Парсит JSON из текстового редактора. Пусто → null. Кидает при невалидном JSON.
  dynamic _parseJsonField(String text) {
    final t = text.trim();
    if (t.isEmpty) {
      return null;
    }
    return jsonDecode(t);
  }

  Future<void> _save() async {
    final l10n = requireAppLocalizations(context, where: 'ScoutConfigForm');
    final messenger = ScaffoldMessenger.of(context);

    // hermes требует явного провайдера.
    if (backendRequiresProvider(_codeBackend) && (_providerKind == null || _providerKind!.isEmpty)) {
      messenger.showSnackBar(SnackBar(content: Text(l10n.scoutProviderRequired)));
      return;
    }

    // Собираем code_backend_settings, сохраняя неизвестные ключи (env/hooks/hermes).
    final cbs = <String, dynamic>{...widget.config.codeBackendSettings};
    final model = _modelCtrl.text.trim();
    if (model.isEmpty) {
      cbs.remove('model');
    } else {
      cbs['model'] = model;
    }
    Map<String, dynamic> perms;
    try {
      final mcp = _parseJsonField(_mcpCtrl.text);
      if (mcp == null) {
        cbs.remove('mcp_servers');
      } else {
        cbs['mcp_servers'] = mcp;
      }
      final skills = _parseJsonField(_skillsCtrl.text);
      if (skills == null) {
        cbs.remove('skills');
      } else {
        cbs['skills'] = skills;
      }
      final p = _parseJsonField(_permsCtrl.text);
      perms = p == null ? <String, dynamic>{} : (p as Map<String, dynamic>);
    } catch (_) {
      messenger.showSnackBar(SnackBar(content: Text(l10n.scoutInvalidJsonSnack)));
      return;
    }

    setState(() => _busy = true);
    try {
      await widget.onSave(
        isEnabled: _isEnabled,
        codeBackend: _codeBackend,
        providerKind: _providerKind ?? '',
        temperature: _temperature,
        codeBackendSettings: cbs,
        sandboxPermissions: perms,
        timeoutSeconds: int.tryParse(_timeoutCtrl.text.trim()),
      );
      if (mounted) {
        setState(() => _dirty = false);
      }
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ScoutConfigForm');
    final theme = Theme.of(context);
    final allowedProviders = allowedProviderKindsForBackend(_codeBackend).toList()..sort();

    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(l10n.scoutEnabledLabel),
              subtitle: Text(l10n.scoutEnabledHint),
              value: _isEnabled,
              onChanged: (v) {
                setState(() => _isEnabled = v);
                _markDirty();
              },
            ),
            const SizedBox(height: 8),
            DropdownButtonFormField<String>(
              initialValue: _codeBackend,
              decoration: InputDecoration(
                labelText: l10n.scoutBackendLabel,
                border: const OutlineInputBorder(),
              ),
              items: [
                for (final b in kSupportedCodeBackends)
                  DropdownMenuItem(value: b, child: Text(b)),
              ],
              onChanged: _onBackendChanged,
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String?>(
              initialValue: allowedProviders.contains(_providerKind) ? _providerKind : null,
              decoration: InputDecoration(
                labelText: l10n.scoutProviderLabel,
                helperText: l10n.scoutProviderHint,
                border: const OutlineInputBorder(),
              ),
              items: [
                if (!backendRequiresProvider(_codeBackend))
                  DropdownMenuItem<String?>(value: null, child: Text(l10n.scoutProviderNone)),
                for (final p in allowedProviders)
                  DropdownMenuItem<String?>(value: p, child: Text(p)),
              ],
              onChanged: (v) {
                setState(() => _providerKind = v);
                _markDirty();
              },
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _modelCtrl,
              decoration: InputDecoration(
                labelText: l10n.scoutModelLabel,
                hintText: l10n.scoutModelHint,
                border: const OutlineInputBorder(),
              ),
              onChanged: (_) => _markDirty(),
            ),
            const SizedBox(height: 16),
            Text(
              '${l10n.scoutTemperatureLabel}: ${_temperature.toStringAsFixed(1)}',
              style: theme.textTheme.labelLarge,
            ),
            Slider(
              value: _temperature,
              min: 0,
              max: 2,
              divisions: 20,
              label: _temperature.toStringAsFixed(1),
              onChanged: (v) {
                setState(() => _temperature = v);
                _markDirty();
              },
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _timeoutCtrl,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(
                labelText: l10n.scoutTimeoutLabel,
                helperText: l10n.scoutTimeoutHint,
                border: const OutlineInputBorder(),
              ),
              onChanged: (_) => _markDirty(),
            ),
            const SizedBox(height: 8),
            Theme(
              data: theme.copyWith(dividerColor: Colors.transparent),
              child: ExpansionTile(
                tilePadding: EdgeInsets.zero,
                childrenPadding: const EdgeInsets.only(top: 8, bottom: 8),
                title: Text(l10n.scoutAdvancedTitle, style: theme.textTheme.titleSmall),
                children: [
                  _JsonField(
                    controller: _mcpCtrl,
                    label: l10n.scoutMcpLabel,
                    hint: l10n.scoutMcpHint,
                    onChanged: _markDirty,
                  ),
                  const SizedBox(height: 12),
                  _JsonField(
                    controller: _skillsCtrl,
                    label: l10n.scoutSkillsLabel,
                    hint: l10n.scoutSkillsHint,
                    onChanged: _markDirty,
                  ),
                  const SizedBox(height: 12),
                  _JsonField(
                    controller: _permsCtrl,
                    label: l10n.scoutPermissionsLabel,
                    hint: l10n.scoutPermissionsHint,
                    onChanged: _markDirty,
                  ),
                ],
              ),
            ),
            const SizedBox(height: 12),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.workspace_premium_outlined,
                    size: 18, color: theme.colorScheme.onSurfaceVariant),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.scoutSubscriptionNote,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton(
                onPressed: (_busy || !_dirty) ? null : _save,
                child: Text(l10n.scoutSaveButton),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Многострочный JSON-редактор (как в диалоге настроек агента).
class _JsonField extends StatelessWidget {
  const _JsonField({
    required this.controller,
    required this.label,
    required this.hint,
    required this.onChanged,
  });

  final TextEditingController controller;
  final String label;
  final String hint;
  final VoidCallback onChanged;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      minLines: 2,
      maxLines: 8,
      style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
      decoration: InputDecoration(
        labelText: label,
        helperText: hint,
        border: const OutlineInputBorder(),
        alignLabelWithHint: true,
      ),
      onChanged: (_) => onChanged(),
    );
  }
}
