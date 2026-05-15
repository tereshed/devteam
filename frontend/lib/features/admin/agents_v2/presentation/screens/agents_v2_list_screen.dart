import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/admin/agents_v2/presentation/widgets/agent_v2_create_dialog.dart';
import 'package:go_router/go_router.dart';

class AgentsV2ListScreen extends ConsumerWidget {
  const AgentsV2ListScreen({super.key});

  Future<void> _openCreate(BuildContext context, WidgetRef ref) async {
    final created = await showDialog<bool>(
      context: context,
      builder: (_) => const AgentV2CreateDialog(),
    );
    if (created == true) {
      ref.invalidate(agentsV2ListProvider);
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AgentsV2ListScreen');
    final agentsAsync = ref.watch(agentsV2ListProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.agentsV2Title),
        actions: [
          IconButton(
            tooltip: l10n.agentsV2Refresh,
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.invalidate(agentsV2ListProvider),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        icon: const Icon(Icons.add),
        label: Text(l10n.agentsV2CreateButton),
        onPressed: () => _openCreate(context, ref),
      ),
      body: agentsAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) =>
            Center(child: Text('${l10n.dataLoadError}: $err')),
        data: (page) {
          if (page.items.isEmpty) {
            return Center(child: Text(l10n.agentsV2Empty));
          }
          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(agentsV2ListProvider);
              await ref.read(agentsV2ListProvider.future);
            },
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: page.items.length,
              separatorBuilder: (_, __) => const Divider(height: 0),
              itemBuilder: (context, index) {
                final agent = page.items[index];
                return _AgentV2Tile(agent: agent);
              },
            ),
          );
        },
      ),
    );
  }
}

class _AgentV2Tile extends StatelessWidget {
  final AgentV2 agent;

  const _AgentV2Tile({required this.agent});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentsV2ListTile');
    final kindLabel = agent.isLlm
        ? l10n.agentsV2KindLlm
        : l10n.agentsV2KindSandbox;
    final subtitle = <String>[
      agent.role,
      kindLabel,
      if (agent.isLlm && agent.model != null) agent.model!,
      if (agent.isSandbox && agent.codeBackend != null) agent.codeBackend!,
    ].where((s) => s.isNotEmpty).join(' · ');

    return ListTile(
      leading: CircleAvatar(
        backgroundColor: agent.isLlm
            ? Colors.deepPurple.shade100
            : Colors.teal.shade100,
        child: Icon(
          agent.isLlm ? Icons.psychology : Icons.terminal,
          color: agent.isLlm ? Colors.deepPurple : Colors.teal,
        ),
      ),
      title: Text(
        agent.name,
        style: const TextStyle(fontWeight: FontWeight.w600),
      ),
      subtitle: Text(subtitle),
      trailing: Icon(
        agent.isActive ? Icons.check_circle : Icons.pause_circle_outline,
        color: agent.isActive ? Colors.green : Colors.grey,
      ),
      onTap: () => context.go('/admin/agents-v2/${agent.id}'),
    );
  }
}
