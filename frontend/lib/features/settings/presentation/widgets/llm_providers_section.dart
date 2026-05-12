import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/settings/data/llm_providers_repository.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';

/// Sprint 15.30 — вкладка «LLM-провайдеры»:
///  - список из llm_providers (с переключателем enabled);
///  - кнопка «Добавить»;
///  - кнопки edit/health-check/delete на каждом провайдере;
///  - чекбокс «использовать free-claude-proxy» при добавлении/редактировании.
class LLMProvidersSection extends ConsumerWidget {
  const LLMProvidersSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersSection');
    final providers = ref.watch(llmProvidersListProvider);

    return providers.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => _ErrorBlock(
        message: l10n.llmProvidersLoadError,
        error: err.toString(),
        onRetry: () => ref.invalidate(llmProvidersListProvider),
      ),
      data: (list) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    l10n.llmProvidersSectionTitle,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                ),
                FilledButton.icon(
                  onPressed: () => _openEditor(context, ref, null),
                  icon: const Icon(Icons.add),
                  label: Text(l10n.llmProvidersAdd),
                ),
              ],
            ),
            const SizedBox(height: 12),
            if (list.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 32),
                child: Center(child: Text(l10n.llmProvidersEmpty)),
              )
            else
              ...list.map((p) => _ProviderRow(
                    provider: p,
                    onEdit: () => _openEditor(context, ref, p),
                    onHealthCheck: () => _healthCheck(context, ref, p),
                    onDelete: () => _delete(context, ref, p),
                  )),
          ],
        );
      },
    );
  }

  Future<void> _openEditor(
    BuildContext context,
    WidgetRef ref,
    LLMProviderModel? existing,
  ) async {
    final saved = await showDialog<bool>(
      context: context,
      builder: (_) => _LLMProviderEditorDialog(existing: existing),
    );
    if (saved == true) {
      ref.invalidate(llmProvidersListProvider);
    }
  }

  Future<void> _healthCheck(
    BuildContext context,
    WidgetRef ref,
    LLMProviderModel p,
  ) async {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersHealth');
    final repo = ref.read(llmProvidersRepositoryProvider);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await repo.healthCheck(p.id);
      messenger.showSnackBar(SnackBar(content: Text(l10n.llmProvidersHealthOK)));
    } catch (err) {
      messenger.showSnackBar(
        SnackBar(content: Text('${l10n.llmProvidersHealthFail}: $err')),
      );
    }
  }

  Future<void> _delete(
    BuildContext context,
    WidgetRef ref,
    LLMProviderModel p,
  ) async {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersDelete');
    final confirm = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(l10n.llmProvidersDeleteTitle),
        content: Text(l10n.llmProvidersDeleteConfirm(p.name)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton.tonal(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l10n.delete),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    final repo = ref.read(llmProvidersRepositoryProvider);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await repo.delete(p.id);
      ref.invalidate(llmProvidersListProvider);
    } catch (err) {
      messenger.showSnackBar(
        SnackBar(content: Text('${l10n.llmProvidersDeleteFail}: $err')),
      );
    }
  }
}

class _ProviderRow extends StatelessWidget {
  const _ProviderRow({
    required this.provider,
    required this.onEdit,
    required this.onHealthCheck,
    required this.onDelete,
  });

  final LLMProviderModel provider;
  final VoidCallback onEdit;
  final VoidCallback onHealthCheck;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersRow');
    return Card(
      child: ListTile(
        leading: Icon(provider.enabled ? Icons.check_circle : Icons.cancel,
            color: provider.enabled ? Colors.green : Colors.grey),
        title: Text(provider.name),
        subtitle: Text('${provider.kind} • ${provider.defaultModel}'),
        trailing: Wrap(
          spacing: 4,
          children: [
            IconButton(
              tooltip: l10n.llmProvidersHealthTooltip,
              icon: const Icon(Icons.health_and_safety_outlined),
              onPressed: onHealthCheck,
            ),
            IconButton(
              tooltip: l10n.llmProvidersEditTooltip,
              icon: const Icon(Icons.edit),
              onPressed: onEdit,
            ),
            IconButton(
              tooltip: l10n.llmProvidersDeleteTooltip,
              icon: const Icon(Icons.delete_outline),
              onPressed: onDelete,
            ),
          ],
        ),
      ),
    );
  }
}

class _ErrorBlock extends StatelessWidget {
  const _ErrorBlock({
    required this.message,
    required this.error,
    required this.onRetry,
  });

  final String message;
  final String error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersError');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 24),
        Text(message, style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 8),
        SelectableText(error),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          onPressed: onRetry,
          icon: const Icon(Icons.refresh),
          label: Text(l10n.retry),
        ),
      ],
    );
  }
}

/// Диалог создания/редактирования LLM-провайдера (Sprint 15.30).
class _LLMProviderEditorDialog extends ConsumerStatefulWidget {
  const _LLMProviderEditorDialog({this.existing});

  final LLMProviderModel? existing;

  @override
  ConsumerState<_LLMProviderEditorDialog> createState() =>
      _LLMProviderEditorDialogState();
}

class _LLMProviderEditorDialogState
    extends ConsumerState<_LLMProviderEditorDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _name;
  late final TextEditingController _baseURL;
  late final TextEditingController _credential;
  late final TextEditingController _defaultModel;
  late String _kind;
  late String _authType;
  late bool _enabled;
  bool _useProxy = false;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _name = TextEditingController(text: e?.name ?? '');
    _baseURL = TextEditingController(text: e?.baseURL ?? '');
    _credential = TextEditingController();
    _defaultModel = TextEditingController(text: e?.defaultModel ?? '');
    _kind = e?.kind ?? 'openrouter';
    _authType = e?.authType ?? 'api_key';
    _enabled = e?.enabled ?? true;
    _useProxy = _kind == 'free_claude_proxy';
  }

  @override
  void dispose() {
    _name.dispose();
    _baseURL.dispose();
    _credential.dispose();
    _defaultModel.dispose();
    super.dispose();
  }

  Future<void> _test() async {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersEditor');
    final repo = ref.read(llmProvidersRepositoryProvider);
    final messenger = ScaffoldMessenger.of(context);
    if (!_formKey.currentState!.validate()) return;
    setState(() => _busy = true);
    try {
      await repo.testConnection(
        name: _name.text.trim(),
        kind: _kind,
        baseURL: _baseURL.text.trim(),
        authType: _authType,
        credential: _credential.text,
        defaultModel: _defaultModel.text.trim(),
      );
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.llmProvidersTestOK)),
      );
    } catch (err) {
      messenger.showSnackBar(
        SnackBar(content: Text('${l10n.llmProvidersTestFail}: $err')),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    final repo = ref.read(llmProvidersRepositoryProvider);
    setState(() => _busy = true);
    try {
      if (widget.existing == null) {
        await repo.create(
          name: _name.text.trim(),
          kind: _kind,
          baseURL: _baseURL.text.trim(),
          authType: _authType,
          credential: _credential.text,
          defaultModel: _defaultModel.text.trim(),
          enabled: _enabled,
        );
      } else {
        await repo.update(
          id: widget.existing!.id,
          name: _name.text.trim(),
          kind: _kind,
          baseURL: _baseURL.text.trim(),
          authType: _authType,
          credential: _credential.text,
          defaultModel: _defaultModel.text.trim(),
          enabled: _enabled,
        );
      }
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } catch (err) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('$err')));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'llmProvidersEditor');
    return AlertDialog(
      title: Text(widget.existing == null
          ? l10n.llmProvidersAddTitle
          : l10n.llmProvidersEditTitle),
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
                  controller: _name,
                  decoration: InputDecoration(
                    labelText: l10n.llmProvidersFieldName,
                  ),
                  validator: (v) =>
                      (v == null || v.trim().isEmpty) ? l10n.fieldRequired : null,
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  initialValue: _kind,
                  decoration: InputDecoration(
                    labelText: l10n.llmProvidersFieldKind,
                  ),
                  items: kSupportedLLMProviderKinds
                      .map((k) => DropdownMenuItem(value: k, child: Text(k)))
                      .toList(),
                  onChanged: (v) => setState(() {
                    _kind = v ?? _kind;
                    _useProxy = _kind == 'free_claude_proxy';
                  }),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _baseURL,
                  decoration: InputDecoration(
                    labelText: l10n.llmProvidersFieldBaseURL,
                  ),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _credential,
                  decoration: InputDecoration(
                    labelText: widget.existing == null
                        ? l10n.llmProvidersFieldCredential
                        : l10n.llmProvidersFieldCredentialOptional,
                  ),
                  obscureText: true,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _defaultModel,
                  decoration: InputDecoration(
                    labelText: l10n.llmProvidersFieldDefaultModel,
                  ),
                ),
                const SizedBox(height: 12),
                Row(
                  children: [
                    Expanded(child: Text(l10n.llmProvidersFieldUseProxy)),
                    Switch(
                      value: _useProxy,
                      onChanged: (v) => setState(() {
                        _useProxy = v;
                        if (v) _kind = 'free_claude_proxy';
                      }),
                    ),
                  ],
                ),
                Row(
                  children: [
                    Expanded(child: Text(l10n.llmProvidersFieldEnabled)),
                    Switch(
                      value: _enabled,
                      onChanged: (v) => setState(() => _enabled = v),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(false),
          child: Text(l10n.cancel),
        ),
        OutlinedButton(
          onPressed: _busy ? null : _test,
          child: Text(l10n.llmProvidersTest),
        ),
        FilledButton(
          onPressed: _busy ? null : _save,
          child: Text(_busy ? '…' : l10n.save),
        ),
      ],
    );
  }
}
