import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_config_model.dart';

/// Колбэк сохранения конфига энхансера (частичный апдейт).
typedef EnhancerConfigSave = Future<void> Function({
  bool? isActive,
  String? cronExpression,
  int? analysisWindowDays,
  int? maxChangesPerRun,
});

/// Форма конфига энхансера: тумблер, режим применения, cron, лимиты.
class EnhancerConfigForm extends StatefulWidget {
  const EnhancerConfigForm({
    super.key,
    required this.config,
    required this.onSave,
  });

  final EnhancerConfigModel config;
  final EnhancerConfigSave onSave;

  @override
  State<EnhancerConfigForm> createState() => _EnhancerConfigFormState();
}

class _EnhancerConfigFormState extends State<EnhancerConfigForm> {
  late bool _isActive;
  late final TextEditingController _cronCtrl;
  late final TextEditingController _windowCtrl;
  late final TextEditingController _maxChangesCtrl;
  bool _busy = false;
  bool _dirty = false;

  @override
  void initState() {
    super.initState();
    _isActive = widget.config.isActive;
    _cronCtrl = TextEditingController(text: widget.config.cronExpression ?? '');
    _windowCtrl = TextEditingController(
      text: widget.config.analysisWindowDays.toString(),
    );
    _maxChangesCtrl = TextEditingController(
      text: widget.config.maxChangesPerRun.toString(),
    );
  }

  @override
  void dispose() {
    _cronCtrl.dispose();
    _windowCtrl.dispose();
    _maxChangesCtrl.dispose();
    super.dispose();
  }

  void _markDirty() {
    if (!_dirty) {
      setState(() => _dirty = true);
    }
  }

  Future<void> _save() async {
    setState(() => _busy = true);
    try {
      await widget.onSave(
        isActive: _isActive,
        cronExpression: _cronCtrl.text.trim(),
        analysisWindowDays: int.tryParse(_windowCtrl.text.trim()),
        maxChangesPerRun: int.tryParse(_maxChangesCtrl.text.trim()),
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
    final l10n = requireAppLocalizations(context, where: 'EnhancerConfigForm');
    final theme = Theme.of(context);

    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            SwitchListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(l10n.enhancerEnabledLabel),
              value: _isActive,
              onChanged: (v) {
                setState(() => _isActive = v);
                _markDirty();
              },
            ),
            const SizedBox(height: 8),
            Text(l10n.enhancerAutonomyLabel,
                style: theme.textTheme.labelLarge),
            const SizedBox(height: 8),
            // Фаза 1: только propose. auto_apply показан, но недоступен —
            // появится вместе с замером эффекта (фаза 3).
            SegmentedButton<String>(
              segments: [
                ButtonSegment(
                  value: 'propose',
                  label: Text(l10n.enhancerAutonomyPropose),
                ),
                ButtonSegment(
                  value: 'auto_apply',
                  enabled: false,
                  label: Tooltip(
                    message: l10n.enhancerAutonomyAutoApplySoon,
                    child: Text(l10n.enhancerAutonomyAutoApply),
                  ),
                ),
              ],
              selected: const {'propose'},
              onSelectionChanged: (_) {},
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _cronCtrl,
              decoration: InputDecoration(
                labelText: l10n.enhancerCronLabel,
                helperText: l10n.enhancerCronHint,
                border: const OutlineInputBorder(),
              ),
              onChanged: (_) => _markDirty(),
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _windowCtrl,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: l10n.enhancerWindowLabel,
                      border: const OutlineInputBorder(),
                    ),
                    onChanged: (_) => _markDirty(),
                  ),
                ),
                const SizedBox(width: 16),
                Expanded(
                  child: TextField(
                    controller: _maxChangesCtrl,
                    keyboardType: TextInputType.number,
                    decoration: InputDecoration(
                      labelText: l10n.enhancerMaxChangesLabel,
                      border: const OutlineInputBorder(),
                    ),
                    onChanged: (_) => _markDirty(),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Align(
              alignment: Alignment.centerRight,
              child: FilledButton(
                onPressed: (_busy || !_dirty) ? null : _save,
                child: Text(l10n.enhancerSaveButton),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
