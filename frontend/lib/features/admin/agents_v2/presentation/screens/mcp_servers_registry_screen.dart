import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/admin/agents_v2/data/mcp_registry_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/mcp_registry_repository.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

class MCPServersRegistryScreen extends ConsumerWidget {
  const MCPServersRegistryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'MCPServersRegistryScreen');
    final registryAsync = ref.watch(mcpRegistryListProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.mcpRegistryScreenTitle),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.invalidate(mcpRegistryListProvider),
            tooltip: l10n.mcpRegistryRefreshTooltip,
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showEditDialog(context, ref, null),
        child: const Icon(Icons.add),
      ),
      body: SafeArea(
        child: AdaptiveContainer(
          child: registryAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => Center(child: Text('${l10n.mcpRegistryLoadError}: $err')),
            data: (servers) {
              if (servers.isEmpty) {
                return Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.dns_outlined, size: 64, color: Theme.of(context).colorScheme.primary.withValues(alpha: 0.5)),
                      const SizedBox(height: 16),
                      Text(l10n.mcpRegistryEmpty, style: Theme.of(context).textTheme.bodyLarge),
                    ],
                  ),
                );
              }
              return ListView.builder(
                padding: Spacing.cardPadding(context),
                itemCount: servers.length,
                itemBuilder: (ctx, i) => _MCPServerTile(
                  server: servers[i],
                  onEdit: () => _showEditDialog(context, ref, servers[i]),
                  onDelete: () => _confirmDelete(context, ref, servers[i]),
                ),
              );
            },
          ),
        ),
      ),
    );
  }

  void _showEditDialog(BuildContext context, WidgetRef ref, MCPServerRegistryModel? server) {
    showDialog(
      context: context,
      builder: (ctx) => _MCPServerEditDialog(
        server: server,
        onSave: (data) async {
          final repo = ref.read(mcpRegistryRepositoryProvider);
          if (server == null) {
            await repo.create(
              name: data['name'] as String,
              transport: data['transport'] as String,
              description: data['description'] as String?,
              command: data['command'] as String?,
              url: data['url'] as String?,
              scope: data['scope'] as String?,
              isActive: data['is_active'] as bool?,
            );
          } else {
            await repo.update(
              server.id,
              name: data['name'] as String,
              transport: data['transport'] as String,
              description: data['description'] as String?,
              command: data['command'] as String?,
              url: data['url'] as String?,
              scope: data['scope'] as String?,
              isActive: data['is_active'] as bool?,
            );
          }
          ref.invalidate(mcpRegistryListProvider);
        },
      ),
    );
  }

  void _confirmDelete(BuildContext context, WidgetRef ref, MCPServerRegistryModel server) {
    final l10n = requireAppLocalizations(context, where: 'MCPServersRegistryScreen');
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.mcpRegistryDeleteTitle),
        content: Text('${l10n.mcpRegistryDeleteConfirm} ${server.name}?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(l10n.mcpRegistryCancelButton),
          ),
          FilledButton(
            onPressed: () async {
              Navigator.of(ctx).pop();
              final repo = ref.read(mcpRegistryRepositoryProvider);
              await repo.delete(server.id);
              ref.invalidate(mcpRegistryListProvider);
            },
            style: FilledButton.styleFrom(backgroundColor: Theme.of(ctx).colorScheme.error),
            child: Text(l10n.mcpRegistryDeleteButton),
          ),
        ],
      ),
    );
  }
}

class _MCPServerTile extends StatelessWidget {
  const _MCPServerTile({required this.server, required this.onEdit, required this.onDelete});
  final MCPServerRegistryModel server;
  final VoidCallback onEdit;
  final VoidCallback onDelete;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      child: ListTile(
        leading: Icon(
          server.transport == 'stdio' ? Icons.terminal : Icons.cloud_outlined,
          color: server.isActive ? theme.colorScheme.primary : theme.colorScheme.onSurface.withValues(alpha: 0.38),
        ),
        title: Text(server.name),
        subtitle: Text('${server.transport} | ${server.scope}'),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (!server.isActive)
              Padding(
                padding: const EdgeInsets.only(right: 8.0),
                child: Chip(
                  label: Text(
                    'inactive',
                    style: theme.textTheme.labelSmall,
                  ),
                  visualDensity: VisualDensity.compact,
                ),
              ),
            IconButton(icon: const Icon(Icons.edit_outlined), onPressed: onEdit),
            IconButton(
              icon: Icon(Icons.delete_outline, color: theme.colorScheme.error),
              onPressed: onDelete,
            ),
          ],
        ),
      ),
    );
  }
}

const _kTransports = ['stdio', 'http', 'sse'];
const _kScopes = ['global', 'project', 'agent'];

class _MCPServerEditDialog extends StatefulWidget {
  const _MCPServerEditDialog({this.server, required this.onSave});
  final MCPServerRegistryModel? server;
  final Future<void> Function(Map<String, dynamic> data) onSave;

  @override
  State<_MCPServerEditDialog> createState() => _MCPServerEditDialogState();
}

class _MCPServerEditDialogState extends State<_MCPServerEditDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _nameCtrl;
  late final TextEditingController _descCtrl;
  late final TextEditingController _commandCtrl;
  late final TextEditingController _urlCtrl;
  late String _transport;
  late String _scope;
  late bool _isActive;
  bool _isSaving = false;

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController(text: widget.server?.name ?? '');
    _descCtrl = TextEditingController(text: widget.server?.description ?? '');
    _commandCtrl = TextEditingController(text: widget.server?.command ?? '');
    _urlCtrl = TextEditingController(text: widget.server?.url ?? '');
    _transport = widget.server?.transport ?? 'stdio';
    _scope = widget.server?.scope ?? 'global';
    _isActive = widget.server?.isActive ?? true;
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _descCtrl.dispose();
    _commandCtrl.dispose();
    _urlCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'MCPServerEditDialog');
    final isNew = widget.server == null;

    return AlertDialog(
      title: Text(isNew ? l10n.mcpRegistryAddTitle : l10n.mcpRegistryEditTitle),
      content: SizedBox(
        width: 480,
        child: Form(
          key: _formKey,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextFormField(
                  controller: _nameCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.mcpRegistryNameLabel,
                    border: const OutlineInputBorder(),
                  ),
                  validator: (v) => (v == null || v.isEmpty) ? l10n.mcpRegistryNameRequired : null,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _descCtrl,
                  decoration: InputDecoration(
                    labelText: l10n.mcpRegistryDescLabel,
                    border: const OutlineInputBorder(),
                  ),
                  maxLines: 2,
                ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: _transport,
                  decoration: InputDecoration(
                    labelText: l10n.mcpRegistryTransportLabel,
                    border: const OutlineInputBorder(),
                  ),
                  items: _kTransports.map((t) => DropdownMenuItem(value: t, child: Text(t))).toList(),
                  onChanged: (v) {
                    if (v != null) setState(() => _transport = v);
                  },
                ),
                const SizedBox(height: 12),
                if (_transport == 'stdio')
                  TextFormField(
                    controller: _commandCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.mcpRegistryCommandLabel,
                      border: const OutlineInputBorder(),
                    ),
                  )
                else
                  TextFormField(
                    controller: _urlCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.mcpRegistryURLLabel,
                      border: const OutlineInputBorder(),
                    ),
                  ),
                const SizedBox(height: 12),
                DropdownButtonFormField<String>(
                  value: _scope,
                  decoration: InputDecoration(
                    labelText: l10n.mcpRegistryScopeLabel,
                    border: const OutlineInputBorder(),
                  ),
                  items: _kScopes.map((s) => DropdownMenuItem(value: s, child: Text(s))).toList(),
                  onChanged: (v) {
                    if (v != null) setState(() => _scope = v);
                  },
                ),
                const SizedBox(height: 12),
                SwitchListTile(
                  title: Text(l10n.mcpRegistryActiveLabel),
                  value: _isActive,
                  onChanged: (v) => setState(() => _isActive = v),
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isSaving ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.mcpRegistryCancelButton),
        ),
        FilledButton(
          onPressed: _isSaving ? null : _submit,
          child: _isSaving
              ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
              : Text(l10n.mcpRegistrySaveButton),
        ),
      ],
    );
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _isSaving = true);
    try {
      await widget.onSave({
        'name': _nameCtrl.text,
        'description': _descCtrl.text,
        'transport': _transport,
        'command': _commandCtrl.text,
        'url': _urlCtrl.text,
        'scope': _scope,
        'is_active': _isActive,
      });
      if (mounted) Navigator.of(context).pop();
    } catch (e) {
      setState(() => _isSaving = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
      }
    }
  }
}
