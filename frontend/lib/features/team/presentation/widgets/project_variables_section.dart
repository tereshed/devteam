import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/team/data/agent_config_providers.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

class ProjectVariablesSection extends ConsumerWidget {
  const ProjectVariablesSection({super.key, required this.projectId});
  final String projectId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    final secretsAsync = ref.watch(projectSecretsProvider(projectId));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l10n.projectVariablesTitle, style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        Text(
          l10n.projectVariablesHint,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.6),
          ),
        ),
        const SizedBox(height: 16),
        secretsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Text('${l10n.projectVariablesLoadError}: $err'),
          data: (secrets) => _SecretsList(
            secrets: secrets,
            onDelete: (secretId) async {
              final repo = ref.read(projectSecretRepositoryProvider);
              await repo.delete(projectId, secretId);
              ref.invalidate(projectSecretsProvider(projectId));
            },
          ),
        ),
        const SizedBox(height: 8),
        _AddSecretButton(
          onAdd: (keyName, value) async {
            final repo = ref.read(projectSecretRepositoryProvider);
            await repo.set(projectId, keyName: keyName, value: value);
            ref.invalidate(projectSecretsProvider(projectId));
          },
        ),
      ],
    );
  }
}

class _SecretsList extends StatelessWidget {
  const _SecretsList({required this.secrets, required this.onDelete});
  final List<SecretRefModel> secrets;
  final Future<void> Function(String secretId) onDelete;

  @override
  Widget build(BuildContext context) {
    if (secrets.isEmpty) {
      final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 8.0),
        child: Text(
          l10n.projectVariablesEmpty,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.6),
          ),
        ),
      );
    }
    return Column(
      children: secrets.map((secret) {
        return ListTile(
          leading: const Icon(Icons.vpn_key_outlined),
          title: Text(secret.keyName),
          subtitle: const Text('••••••••'),
          trailing: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              IconButton(
                icon: const Icon(Icons.edit_outlined, size: 20),
                onPressed: () => _showEditDialog(context, secret),
              ),
              IconButton(
                icon: Icon(Icons.delete_outline, size: 20, color: Theme.of(context).colorScheme.error),
                onPressed: () => _confirmDelete(context, secret),
              ),
            ],
          ),
          dense: true,
        );
      }).toList(),
    );
  }

  void _showEditDialog(BuildContext context, SecretRefModel secret) {
    // Edit re-uses the add dialog with the key pre-filled
    showDialog(
      context: context,
      builder: (ctx) => _AddSecretDialog(
        existingKeyName: secret.keyName,
        onSave: (keyName, value) async {
          await onDelete(secret.id);
          // Re-add with new value handled by parent via invalidation
        },
      ),
    );
  }

  void _confirmDelete(BuildContext context, SecretRefModel secret) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.projectVariablesDeleteTitle),
        content: Text('${l10n.projectVariablesDeleteConfirm} ${secret.keyName}?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(l10n.projectVariablesCancelButton),
          ),
          FilledButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              onDelete(secret.id);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(ctx).colorScheme.error,
            ),
            child: Text(l10n.projectVariablesDeleteButton),
          ),
        ],
      ),
    );
  }
}

class _AddSecretButton extends StatelessWidget {
  const _AddSecretButton({required this.onAdd});
  final Future<void> Function(String keyName, String value) onAdd;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    return OutlinedButton.icon(
      onPressed: () {
        showDialog(
          context: context,
          builder: (ctx) => _AddSecretDialog(
            onSave: onAdd,
          ),
        );
      },
      icon: const Icon(Icons.add),
      label: Text(l10n.projectVariablesAddButton),
    );
  }
}

final _keyNameRegex = RegExp(r'^[A-Z][A-Z0-9_]{0,127}$');

class _AddSecretDialog extends StatefulWidget {
  const _AddSecretDialog({this.existingKeyName, required this.onSave});
  final String? existingKeyName;
  final Future<void> Function(String keyName, String value) onSave;

  @override
  State<_AddSecretDialog> createState() => _AddSecretDialogState();
}

class _AddSecretDialogState extends State<_AddSecretDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _keyCtrl;
  final _valueCtrl = TextEditingController();
  bool _isSaving = false;
  bool _obscureValue = true;

  @override
  void initState() {
    super.initState();
    _keyCtrl = TextEditingController(text: widget.existingKeyName ?? '');
  }

  @override
  void dispose() {
    _keyCtrl.dispose();
    _valueCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    return AlertDialog(
      title: Text(widget.existingKeyName != null
          ? l10n.projectVariablesEditTitle
          : l10n.projectVariablesAddTitle),
      content: Form(
        key: _formKey,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextFormField(
              controller: _keyCtrl,
              enabled: widget.existingKeyName == null,
              decoration: InputDecoration(
                labelText: l10n.projectVariablesKeyLabel,
                hintText: 'GITHUB_TOKEN',
                border: const OutlineInputBorder(),
              ),
              textCapitalization: TextCapitalization.characters,
              validator: (v) {
                if (v == null || v.isEmpty) {
                  return l10n.projectVariablesKeyRequired;
                }
                if (!_keyNameRegex.hasMatch(v)) {
                  return l10n.projectVariablesKeyInvalid;
                }
                return null;
              },
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _valueCtrl,
              obscureText: _obscureValue,
              decoration: InputDecoration(
                labelText: l10n.projectVariablesValueLabel,
                border: const OutlineInputBorder(),
                suffixIcon: IconButton(
                  icon: Icon(_obscureValue ? Icons.visibility : Icons.visibility_off),
                  onPressed: () => setState(() => _obscureValue = !_obscureValue),
                ),
              ),
              validator: (v) {
                if (v == null || v.isEmpty) {
                  return l10n.projectVariablesValueRequired;
                }
                return null;
              },
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSaving ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.projectVariablesCancelButton),
        ),
        FilledButton(
          onPressed: _isSaving ? null : _submit,
          child: _isSaving
              ? const SizedBox(
                  width: 16, height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.projectVariablesSaveButton),
        ),
      ],
    );
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isSaving = true);
    try {
      await widget.onSave(_keyCtrl.text, _valueCtrl.text);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      setState(() => _isSaving = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(e.toString())),
        );
      }
    }
  }
}
