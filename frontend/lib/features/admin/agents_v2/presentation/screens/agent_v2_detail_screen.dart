import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/admin/agents_v2/presentation/widgets/agent_v2_secret_dialog.dart';
import 'package:frontend/core/l10n/require.dart';

class AgentV2DetailScreen extends ConsumerStatefulWidget {
  const AgentV2DetailScreen({super.key, required this.agentId});

  final String agentId;

  @override
  ConsumerState<AgentV2DetailScreen> createState() =>
      _AgentV2DetailScreenState();
}

class _AgentV2DetailScreenState extends ConsumerState<AgentV2DetailScreen> {
  final _roleDescCtrl = TextEditingController();
  final _systemPromptCtrl = TextEditingController();
  final _modelCtrl = TextEditingController();
  final _tempCtrl = TextEditingController();
  final _maxTokensCtrl = TextEditingController();
  String? _codeBackend;
  bool _isActive = true;
  bool _hydrated = false;
  bool _saving = false;
  String? _error;

  @override
  void dispose() {
    _roleDescCtrl.dispose();
    _systemPromptCtrl.dispose();
    _modelCtrl.dispose();
    _tempCtrl.dispose();
    _maxTokensCtrl.dispose();
    super.dispose();
  }

  void _hydrate(AgentV2 a) {
    if (_hydrated) return;
    _roleDescCtrl.text = a.roleDescription;
    _systemPromptCtrl.text = a.systemPrompt ?? '';
    _modelCtrl.text = a.model ?? '';
    _tempCtrl.text = a.temperature?.toString() ?? '';
    _maxTokensCtrl.text = a.maxTokens?.toString() ?? '';
    _codeBackend = a.codeBackend;
    _isActive = a.isActive;
    _hydrated = true;
  }

  Future<void> _save(AgentV2 a) async {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_detail_screen');
    setState(() {
      _saving = true;
      _error = null;
    });
    try {
      await ref.read(agentsV2RepositoryProvider).update(
            id: a.id,
            roleDescription: _roleDescCtrl.text,
            systemPrompt: _systemPromptCtrl.text,
            model: a.isLlm ? _modelCtrl.text.trim() : null,
            codeBackend: a.isSandbox ? _codeBackend : null,
            temperature: _tempCtrl.text.trim().isEmpty
                ? null
                : double.tryParse(_tempCtrl.text.trim()),
            maxTokens: _maxTokensCtrl.text.trim().isEmpty
                ? null
                : int.tryParse(_maxTokensCtrl.text.trim()),
            isActive: _isActive,
          );
      ref.invalidate(agentV2DetailProvider(a.id));
      ref.invalidate(agentsV2ListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.agentsV2SavedSnackbar)),
        );
      }
    } catch (e) {
      setState(() => _error = '${l10n.commonRequestFailed}: $e');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _openSecretDialog(AgentV2 a) async {
    final created = await showDialog<bool>(
      context: context,
      builder: (_) => AgentV2SecretDialog(agentId: a.id),
    );
    if (created == true && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
            content: Text(requireAppLocalizations(context, where: 'agent_v2_detail_screen').agentsV2SecretSaved)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_detail_screen');
    final agentAsync = ref.watch(agentV2DetailProvider(widget.agentId));
    return Scaffold(
      appBar: AppBar(title: Text(l10n.agentsV2DetailTitle)),
      body: agentAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) =>
            Center(child: Text('${l10n.dataLoadError}: $err')),
        data: (agent) {
          _hydrate(agent);
          final theme = Theme.of(context);
          return SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                _headerCard(context, agent),
                const SizedBox(height: 16),
                Text(l10n.agentsV2SectionConfig,
                    style: theme.textTheme.titleMedium),
                const SizedBox(height: 12),
                TextField(
                  controller: _roleDescCtrl,
                  decoration: InputDecoration(
                      labelText: l10n.agentsV2FieldRoleDescription),
                  maxLines: 2,
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _systemPromptCtrl,
                  decoration: InputDecoration(
                      labelText: l10n.agentsV2FieldSystemPrompt),
                  maxLines: 12,
                  minLines: 4,
                ),
                if (agent.isLlm) ...[
                  const SizedBox(height: 12),
                  TextField(
                    controller: _modelCtrl,
                    decoration: InputDecoration(
                        labelText: l10n.agentsV2FieldModel),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: TextField(
                          controller: _tempCtrl,
                          decoration: InputDecoration(
                              labelText: l10n.agentsV2FieldTemperature),
                          keyboardType: const TextInputType.numberWithOptions(
                              decimal: true),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Expanded(
                        child: TextField(
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
                    onChanged: (v) => setState(() => _codeBackend = v),
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
                const SizedBox(height: 16),
                Row(
                  children: [
                    FilledButton.icon(
                      key: const Key('agent_v2_detail_save_button'),
                      onPressed: _saving ? null : () => _save(agent),
                      icon: _saving
                          ? const SizedBox(
                              width: 16,
                              height: 16,
                              child:
                                  CircularProgressIndicator(strokeWidth: 2))
                          : const Icon(Icons.save),
                      label: Text(l10n.commonSave),
                    ),
                    const SizedBox(width: 12),
                    OutlinedButton.icon(
                      key: const Key('agent_v2_detail_add_secret_button'),
                      onPressed: () => _openSecretDialog(agent),
                      icon: const Icon(Icons.vpn_key),
                      label: Text(l10n.agentsV2AddSecretButton),
                    ),
                  ],
                ),
                const SizedBox(height: 24),
                _secretsHint(context),
              ],
            ),
          );
        },
      ),
    );
  }

  Widget _headerCard(BuildContext context, AgentV2 a) {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_detail_screen');
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(a.name, style: Theme.of(context).textTheme.titleLarge),
                const SizedBox(width: 12),
                Chip(
                  label: Text(a.executionKind),
                  backgroundColor: a.isLlm
                      ? Colors.deepPurple.shade50
                      : Colors.teal.shade50,
                ),
                const SizedBox(width: 8),
                Chip(label: Text(a.role)),
              ],
            ),
            const SizedBox(height: 8),
            Text('${l10n.agentsV2FieldId}: ${a.id}',
                style: Theme.of(context).textTheme.bodySmall),
          ],
        ),
      ),
    );
  }

  Widget _secretsHint(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agent_v2_detail_screen');
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        border: Border.all(color: Colors.amber.shade300),
        color: Colors.amber.withValues(alpha: 0.07),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          const Icon(Icons.info_outline, size: 18),
          const SizedBox(width: 8),
          Expanded(
            child: Text(l10n.agentsV2SecretsHint,
                style: Theme.of(context).textTheme.bodySmall),
          ),
        ],
      ),
    );
  }
}
