import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/team/data/agent_config_providers.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

/// Колбэк сохранения секрета проекта (upsert по key_name).
typedef SecretSaveCallback = Future<void> Function(
  String keyName,
  String value,
  bool injectAsEnv,
  String description,
);

class ProjectVariablesSection extends ConsumerWidget {
  const ProjectVariablesSection({super.key, required this.projectId});
  final String projectId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    final secretsAsync = ref.watch(projectSecretsProvider(projectId));

    Future<void> save(String keyName, String value, bool injectAsEnv, String description) async {
      final repo = ref.read(projectSecretRepositoryProvider);
      await repo.set(
        projectId,
        keyName: keyName,
        value: value,
        injectAsEnv: injectAsEnv,
        description: description,
      );
      ref.invalidate(projectSecretsProvider(projectId));
    }

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
            onSave: save,
            onDelete: (secretId) async {
              final repo = ref.read(projectSecretRepositoryProvider);
              await repo.delete(projectId, secretId);
              ref.invalidate(projectSecretsProvider(projectId));
            },
          ),
        ),
        const SizedBox(height: 8),
        _AddSecretButton(onSave: save),
      ],
    );
  }
}

class _SecretsList extends StatelessWidget {
  const _SecretsList({required this.secrets, required this.onSave, required this.onDelete});
  final List<SecretRefModel> secrets;
  final SecretSaveCallback onSave;
  final Future<void> Function(String secretId) onDelete;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    if (secrets.isEmpty) {
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
          title: Row(
            children: [
              Flexible(child: Text(secret.keyName, overflow: TextOverflow.ellipsis)),
              if (secret.injectAsEnv) ...[
                const SizedBox(width: 8),
                _EnvBadge(label: l10n.projectVariablesEnvBadge),
              ],
            ],
          ),
          subtitle: Text(
            secret.description.isNotEmpty ? secret.description : '••••••••',
          ),
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
    // Edit переиспользует add-диалог: key_name заблокирован, флаг/описание предзаполнены.
    // Сохранение — upsert через onSave (backend перезаписывает значение по key_name).
    showDialog(
      context: context,
      builder: (ctx) => _AddSecretDialog(
        existingKeyName: secret.keyName,
        existingInjectAsEnv: secret.injectAsEnv,
        existingDescription: secret.description,
        onSave: onSave,
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

class _EnvBadge extends StatelessWidget {
  const _EnvBadge({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 1),
      decoration: BoxDecoration(
        color: scheme.primaryContainer,
        borderRadius: BorderRadius.circular(4),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
          color: scheme.onPrimaryContainer,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class _AddSecretButton extends StatelessWidget {
  const _AddSecretButton({required this.onSave});
  final SecretSaveCallback onSave;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ProjectVariablesSection');
    return OutlinedButton.icon(
      onPressed: () {
        showDialog(
          context: context,
          builder: (ctx) => _AddSecretDialog(onSave: onSave),
        );
      },
      icon: const Icon(Icons.add),
      label: Text(l10n.projectVariablesAddButton),
    );
  }
}

final _keyNameRegex = RegExp(r'^[A-Z][A-Z0-9_]{0,127}$');

class _AddSecretDialog extends StatefulWidget {
  const _AddSecretDialog({
    this.existingKeyName,
    this.existingInjectAsEnv = false,
    this.existingDescription = '',
    required this.onSave,
  });
  final String? existingKeyName;
  final bool existingInjectAsEnv;
  final String existingDescription;
  final SecretSaveCallback onSave;

  @override
  State<_AddSecretDialog> createState() => _AddSecretDialogState();
}

class _AddSecretDialogState extends State<_AddSecretDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _keyCtrl;
  late final TextEditingController _descCtrl;
  final _valueCtrl = TextEditingController();
  bool _isSaving = false;
  bool _obscureValue = true;
  late bool _injectAsEnv;

  @override
  void initState() {
    super.initState();
    _keyCtrl = TextEditingController(text: widget.existingKeyName ?? '');
    _descCtrl = TextEditingController(text: widget.existingDescription);
    _injectAsEnv = widget.existingInjectAsEnv;
  }

  @override
  void dispose() {
    _keyCtrl.dispose();
    _descCtrl.dispose();
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
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                controller: _keyCtrl,
                enabled: widget.existingKeyName == null,
                decoration: InputDecoration(
                  labelText: l10n.projectVariablesKeyLabel,
                  hintText: 'DATABASE_URL',
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
              const SizedBox(height: 16),
              TextFormField(
                controller: _descCtrl,
                decoration: InputDecoration(
                  labelText: l10n.projectVariablesDescriptionLabel,
                  border: const OutlineInputBorder(),
                ),
                maxLength: 255,
              ),
              const SizedBox(height: 8),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: _injectAsEnv,
                onChanged: _isSaving ? null : (v) => setState(() => _injectAsEnv = v),
                title: Text(l10n.projectVariablesInjectLabel),
                subtitle: Text(
                  l10n.projectVariablesInjectHint,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
            ],
          ),
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
    if (!_formKey.currentState!.validate()) {
      return;
    }
    setState(() => _isSaving = true);
    try {
      await widget.onSave(_keyCtrl.text, _valueCtrl.text, _injectAsEnv, _descCtrl.text.trim());
      if (mounted) {
        Navigator.of(context).pop();
      }
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
