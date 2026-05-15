import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/core/l10n/require.dart';

class AgentV2SecretDialog extends ConsumerStatefulWidget {
  const AgentV2SecretDialog({super.key, required this.agentId});

  final String agentId;

  @override
  ConsumerState<AgentV2SecretDialog> createState() =>
      _AgentV2SecretDialogState();
}

class _AgentV2SecretDialogState extends ConsumerState<AgentV2SecretDialog> {
  final _formKey = GlobalKey<FormState>();
  final _keyCtrl = TextEditingController();
  final _valueCtrl = TextEditingController();
  bool _obscure = true;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _keyCtrl.dispose();
    _valueCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_secret_dialog');
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(agentsV2RepositoryProvider).setSecret(
            agentId: widget.agentId,
            keyName: _keyCtrl.text.trim(),
            value: _valueCtrl.text,
          );
      if (mounted) Navigator.of(context).pop(true);
    } catch (e) {
      setState(() => _error = '${l10n.commonRequestFailed}: $e');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_secret_dialog');
    return AlertDialog(
      title: Text(l10n.agentsV2SecretDialogTitle),
      content: SizedBox(
        width: 420,
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                controller: _keyCtrl,
                decoration: InputDecoration(
                  labelText: l10n.agentsV2SecretKeyName,
                  hintText: 'GITHUB_TOKEN',
                ),
                validator: (v) =>
                    (v == null || v.trim().isEmpty) ? l10n.commonRequiredField : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _valueCtrl,
                decoration: InputDecoration(
                  labelText: l10n.agentsV2SecretValue,
                  helperText: l10n.agentsV2SecretValueHelper,
                  suffixIcon: IconButton(
                    icon: Icon(
                        _obscure ? Icons.visibility : Icons.visibility_off),
                    onPressed: () => setState(() => _obscure = !_obscure),
                  ),
                ),
                obscureText: _obscure,
                validator: (v) =>
                    (v == null || v.isEmpty) ? l10n.commonRequiredField : null,
              ),
              if (_error != null) ...[
                const SizedBox(height: 8),
                Text(_error!, style: const TextStyle(color: Colors.red)),
              ],
            ],
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
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2))
              : Text(l10n.commonSave),
        ),
      ],
    );
  }
}
