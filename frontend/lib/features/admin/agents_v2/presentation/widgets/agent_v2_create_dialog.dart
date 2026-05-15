import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/core/l10n/require.dart';

class AgentV2CreateDialog extends ConsumerStatefulWidget {
  const AgentV2CreateDialog({super.key});

  @override
  ConsumerState<AgentV2CreateDialog> createState() =>
      _AgentV2CreateDialogState();
}

class _AgentV2CreateDialogState extends ConsumerState<AgentV2CreateDialog> {
  final _formKey = GlobalKey<FormState>();
  final _nameCtrl = TextEditingController();
  final _roleDescCtrl = TextEditingController();
  final _systemPromptCtrl = TextEditingController();
  final _modelCtrl = TextEditingController();
  final _tempCtrl = TextEditingController();
  final _maxTokensCtrl = TextEditingController();

  // Реестр ролей соответствует AgentRole в backend/internal/models.
  static const _roles = <String>[
    'router',
    'planner',
    'decomposer',
    'reviewer',
    'developer',
    'merger',
    'tester',
  ];
  String _role = 'developer';
  String _executionKind = 'llm'; // 'llm' | 'sandbox'
  String _codeBackend = 'claude-code'; // claude-code|aider|hermes|custom
  bool _isActive = true;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _nameCtrl.dispose();
    _roleDescCtrl.dispose();
    _systemPromptCtrl.dispose();
    _modelCtrl.dispose();
    _tempCtrl.dispose();
    _maxTokensCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_create_dialog');
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    final repo = ref.read(agentsV2RepositoryProvider);
    final temperature = _tempCtrl.text.trim().isEmpty
        ? null
        : double.tryParse(_tempCtrl.text.trim());
    final maxTokens = _maxTokensCtrl.text.trim().isEmpty
        ? null
        : int.tryParse(_maxTokensCtrl.text.trim());
    try {
      await repo.create(
        name: _nameCtrl.text.trim(),
        role: _role,
        executionKind: _executionKind,
        roleDescription: _roleDescCtrl.text.trim().isEmpty
            ? null
            : _roleDescCtrl.text.trim(),
        systemPrompt: _systemPromptCtrl.text.trim().isEmpty
            ? null
            : _systemPromptCtrl.text,
        model: _executionKind == 'llm' ? _modelCtrl.text.trim() : null,
        codeBackend: _executionKind == 'sandbox' ? _codeBackend : null,
        temperature: temperature,
        maxTokens: maxTokens,
        isActive: _isActive,
      );
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      setState(() {
        _error = '${l10n.commonRequestFailed}: $e';
      });
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_create_dialog');
    final isLlm = _executionKind == 'llm';

    return AlertDialog(
      title: Text(l10n.agentsV2CreateTitle),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextFormField(
                  controller: _nameCtrl,
                  decoration: InputDecoration(labelText: l10n.agentsV2FieldName),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? l10n.commonRequiredField : null,
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _role,
                  decoration: InputDecoration(labelText: l10n.agentsV2FieldRole),
                  items: _roles
                      .map((r) => DropdownMenuItem(value: r, child: Text(r)))
                      .toList(),
                  onChanged: (v) => setState(() => _role = v ?? 'developer'),
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _executionKind,
                  decoration: InputDecoration(
                      labelText: l10n.agentsV2FieldExecutionKind),
                  items: const [
                    DropdownMenuItem(value: 'llm', child: Text('llm')),
                    DropdownMenuItem(value: 'sandbox', child: Text('sandbox')),
                  ],
                  onChanged: (v) =>
                      setState(() => _executionKind = v ?? 'llm'),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _roleDescCtrl,
                  decoration: InputDecoration(
                      labelText: l10n.agentsV2FieldRoleDescription),
                  maxLines: 2,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _systemPromptCtrl,
                  decoration: InputDecoration(
                      labelText: l10n.agentsV2FieldSystemPrompt),
                  maxLines: 6,
                  minLines: 3,
                ),
                if (isLlm) ...[
                  const SizedBox(height: 12),
                  TextFormField(
                    controller: _modelCtrl,
                    decoration: InputDecoration(
                        labelText: l10n.agentsV2FieldModel,
                        hintText: 'claude-sonnet-4-6'),
                    validator: (v) => isLlm && (v == null || v.trim().isEmpty)
                        ? l10n.commonRequiredField
                        : null,
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: TextFormField(
                          controller: _tempCtrl,
                          decoration: InputDecoration(
                              labelText: l10n.agentsV2FieldTemperature),
                          keyboardType: const TextInputType.numberWithOptions(
                              decimal: true),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: TextFormField(
                          controller: _maxTokensCtrl,
                          decoration: InputDecoration(
                              labelText: l10n.agentsV2FieldMaxTokens),
                          keyboardType: TextInputType.number,
                        ),
                      ),
                    ],
                  ),
                ] else ...[
                  const SizedBox(height: 12),
                  DropdownButtonFormField<String>(
                    initialValue: _codeBackend,
                    decoration: InputDecoration(
                        labelText: l10n.agentsV2FieldCodeBackend),
                    items: const [
                      DropdownMenuItem(
                          value: 'claude-code', child: Text('claude-code')),
                      DropdownMenuItem(value: 'aider', child: Text('aider')),
                      DropdownMenuItem(value: 'hermes', child: Text('hermes')),
                      DropdownMenuItem(value: 'custom', child: Text('custom')),
                    ],
                    onChanged: (v) =>
                        setState(() => _codeBackend = v ?? 'claude-code'),
                  ),
                ],
                const SizedBox(height: 12),
                SwitchListTile.adaptive(
                  contentPadding: EdgeInsets.zero,
                  title: Text(l10n.agentsV2FieldIsActive),
                  value: _isActive,
                  onChanged: (v) => setState(() => _isActive = v),
                ),
                if (_error != null) ...[
                  const SizedBox(height: 8),
                  Text(_error!, style: const TextStyle(color: Colors.red)),
                ],
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.of(context).pop(false),
          child: Text(l10n.commonCancel),
        ),
        FilledButton(
          onPressed: _saving ? null : _submit,
          child: _saving
              ? const SizedBox(
                  width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : Text(l10n.commonCreate),
        ),
      ],
    );
  }
}
