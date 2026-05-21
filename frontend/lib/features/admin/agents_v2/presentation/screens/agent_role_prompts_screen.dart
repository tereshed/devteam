import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/admin/agents_v2/data/role_prompts_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/role_prompts_repository.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';

class AgentRolePromptsScreen extends ConsumerWidget {
  const AgentRolePromptsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AgentRolePromptsScreen');
    final promptsAsync = ref.watch(rolePromptsListProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.rolePromptsScreenTitle),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.invalidate(rolePromptsListProvider),
            tooltip: l10n.rolePromptsRefreshTooltip,
          ),
        ],
      ),
      body: SafeArea(
        child: AdaptiveContainer(
          child: promptsAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => Center(child: Text('${l10n.rolePromptsLoadError}: $err')),
            data: (prompts) {
              if (prompts.isEmpty) {
                return Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.description_outlined, size: 64, color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.5)),
                      const SizedBox(height: 16),
                      Text(l10n.rolePromptsEmpty, style: Theme.of(context).textTheme.bodyLarge),
                    ],
                  ),
                );
              }
              return ListView.builder(
                padding: Spacing.cardPadding(context),
                itemCount: prompts.length,
                itemBuilder: (ctx, i) => _RolePromptTile(
                  prompt: prompts[i],
                  onEdit: () => _showEditDialog(context, ref, prompts[i]),
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  void _showEditDialog(BuildContext context, WidgetRef ref, AgentRolePromptModel prompt) {
    showDialog(
      context: context,
      builder: (ctx) => _RolePromptEditDialog(
        prompt: prompt,
        onSave: (content, description) async {
          final repo = ref.read(rolePromptsRepositoryProvider);
          await repo.update(prompt.role, content: content, description: description);
          ref.invalidate(rolePromptsListProvider);
        },
      ),
    );
  }
}

class _RolePromptTile extends StatelessWidget {
  const _RolePromptTile({required this.prompt, required this.onEdit});
  final AgentRolePromptModel prompt;
  final VoidCallback onEdit;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final preview = prompt.content.length > 100
        ? '${prompt.content.substring(0, 100)}...'
        : prompt.content;

    return Card(
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor: theme.colorScheme.primaryContainer,
          child: Icon(Icons.person_outline, color: theme.colorScheme.onPrimaryContainer),
        ),
        title: Text(prompt.role),
        subtitle: Text(
          preview,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurface.withValues(alpha: 0.6),
          ),
        ),
        trailing: IconButton(
          icon: const Icon(Icons.edit_outlined),
          onPressed: onEdit,
        ),
        isThreeLine: true,
        onTap: onEdit,
      ),
    );
  }
}

class _RolePromptEditDialog extends StatefulWidget {
  const _RolePromptEditDialog({required this.prompt, required this.onSave});
  final AgentRolePromptModel prompt;
  final Future<void> Function(String content, String? description) onSave;

  @override
  State<_RolePromptEditDialog> createState() => _RolePromptEditDialogState();
}

class _RolePromptEditDialogState extends State<_RolePromptEditDialog> {
  late final TextEditingController _contentCtrl;
  late final TextEditingController _descCtrl;
  bool _isSaving = false;

  @override
  void initState() {
    super.initState();
    _contentCtrl = TextEditingController(text: widget.prompt.content);
    _descCtrl = TextEditingController(text: widget.prompt.description ?? '');
  }

  @override
  void dispose() {
    _contentCtrl.dispose();
    _descCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'RolePromptEditDialog');
    return AlertDialog(
      title: Text('${l10n.rolePromptsEditTitle}: ${widget.prompt.role}'),
      content: SizedBox(
        width: 600,
        height: 400,
        child: Column(
          children: [
            Text(
              widget.prompt.role,
              style: Theme.of(context).textTheme.titleSmall?.copyWith(
                color: Theme.of(context).colorScheme.primary,
              ),
            ),
            const SizedBox(height: 8),
            if (widget.prompt.description != null)
              Text(
                widget.prompt.description!,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            const SizedBox(height: 16),
            Expanded(
              child: TextFormField(
                controller: _contentCtrl,
                maxLines: null,
                expands: true,
                textAlignVertical: TextAlignVertical.top,
                decoration: InputDecoration(
                  labelText: l10n.rolePromptsContentLabel,
                  alignLabelWithHint: true,
                  border: const OutlineInputBorder(),
                ),
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSaving ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.rolePromptsCancelButton),
        ),
        FilledButton(
          onPressed: _isSaving ? null : _save,
          child: _isSaving
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : Text(l10n.rolePromptsSaveButton),
        ),
      ],
    );
  }

  Future<void> _save() async {
    if (_contentCtrl.text.isEmpty) return;
    setState(() => _isSaving = true);
    try {
      final desc = _descCtrl.text.isEmpty ? null : _descCtrl.text;
      await widget.onSave(_contentCtrl.text, desc);
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      setState(() => _isSaving = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }
}
