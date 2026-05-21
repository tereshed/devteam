import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/team/data/agent_config_providers.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

class UserVariablesSection extends ConsumerWidget {
  const UserVariablesSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'UserVariablesSection');
    final secretsAsync = ref.watch(userSecretsProvider);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l10n.userVariablesTitle, style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        Text(
          l10n.userVariablesHint,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.6),
          ),
        ),
        const SizedBox(height: 16),
        secretsAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Text('${l10n.userVariablesLoadError}: $err'),
          data: (secrets) => _UserSecretsList(
            secrets: secrets,
            onDelete: (secretId) async {
              final repo = ref.read(userSecretRepositoryProvider);
              await repo.delete(secretId);
              ref.invalidate(userSecretsProvider);
            },
          ),
        ),
        const SizedBox(height: 8),
        _AddUserSecretButton(
          onAdd: (keyName, value) async {
            final repo = ref.read(userSecretRepositoryProvider);
            await repo.set(keyName: keyName, value: value);
            ref.invalidate(userSecretsProvider);
          },
        ),
      ],
    );
  }
}

class _UserSecretsList extends StatelessWidget {
  const _UserSecretsList({required this.secrets, required this.onDelete});
  final List<SecretRefModel> secrets;
  final Future<void> Function(String secretId) onDelete;

  @override
  Widget build(BuildContext context) {
    if (secrets.isEmpty) {
      final l10n = requireAppLocalizations(context, where: 'UserVariablesSection');
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 8.0),
        child: Text(
          l10n.userVariablesEmpty,
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

  void _confirmDelete(BuildContext context, SecretRefModel secret) {
    final l10n = requireAppLocalizations(context, where: 'UserVariablesSection');
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.userVariablesDeleteTitle),
        content: Text('${l10n.userVariablesDeleteConfirm} ${secret.keyName}?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(l10n.userVariablesCancelButton),
          ),
          FilledButton(
            onPressed: () {
              Navigator.of(ctx).pop();
              onDelete(secret.id);
            },
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(ctx).colorScheme.error,
            ),
            child: Text(l10n.userVariablesDeleteButton),
          ),
        ],
      ),
    );
  }
}

final _keyNameRegex = RegExp(r'^[A-Z][A-Z0-9_]{0,127}$');

class _AddUserSecretButton extends StatelessWidget {
  const _AddUserSecretButton({required this.onAdd});
  final Future<void> Function(String keyName, String value) onAdd;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'UserVariablesSection');
    return OutlinedButton.icon(
      onPressed: () {
        showDialog(
          context: context,
          builder: (ctx) => _AddUserSecretDialog(onSave: onAdd),
        );
      },
      icon: const Icon(Icons.add),
      label: Text(l10n.userVariablesAddButton),
    );
  }
}

class _AddUserSecretDialog extends StatefulWidget {
  const _AddUserSecretDialog({required this.onSave});
  final Future<void> Function(String keyName, String value) onSave;

  @override
  State<_AddUserSecretDialog> createState() => _AddUserSecretDialogState();
}

class _AddUserSecretDialogState extends State<_AddUserSecretDialog> {
  final _formKey = GlobalKey<FormState>();
  final _keyCtrl = TextEditingController();
  final _valueCtrl = TextEditingController();
  bool _isSaving = false;
  bool _obscureValue = true;

  @override
  void dispose() {
    _keyCtrl.dispose();
    _valueCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'UserVariablesSection');
    return AlertDialog(
      title: Text(l10n.userVariablesAddTitle),
      content: Form(
        key: _formKey,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextFormField(
              controller: _keyCtrl,
              decoration: InputDecoration(
                labelText: l10n.userVariablesKeyLabel,
                hintText: 'PERSONAL_API_KEY',
                border: const OutlineInputBorder(),
              ),
              textCapitalization: TextCapitalization.characters,
              validator: (v) {
                if (v == null || v.isEmpty) return l10n.userVariablesKeyRequired;
                if (!_keyNameRegex.hasMatch(v)) return l10n.userVariablesKeyInvalid;
                return null;
              },
            ),
            const SizedBox(height: 16),
            TextFormField(
              controller: _valueCtrl,
              obscureText: _obscureValue,
              decoration: InputDecoration(
                labelText: l10n.userVariablesValueLabel,
                border: const OutlineInputBorder(),
                suffixIcon: IconButton(
                  icon: Icon(_obscureValue ? Icons.visibility : Icons.visibility_off),
                  onPressed: () => setState(() => _obscureValue = !_obscureValue),
                ),
              ),
              validator: (v) {
                if (v == null || v.isEmpty) return l10n.userVariablesValueRequired;
                return null;
              },
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSaving ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.userVariablesCancelButton),
        ),
        FilledButton(
          onPressed: _isSaving ? null : _submit,
          child: _isSaving
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : Text(l10n.userVariablesSaveButton),
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
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }
}
