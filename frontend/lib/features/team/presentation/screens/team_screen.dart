import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';

String _agentDisplayName(AppLocalizations l10n, String name) {
  final t = name.trim();
  return t.isEmpty ? l10n.teamAgentNameUnset : t;
}

String _agentModelLine(AppLocalizations l10n, String? model) {
  final m = model?.trim();
  if (m == null || m.isEmpty) {
    return l10n.teamAgentModelUnset;
  }
  return m;
}

String? _optionalLine(String? value) {
  final v = value?.trim();
  if (v == null || v.isEmpty) {
    return null;
  }
  return v;
}

/// Вкладка «Команда»: состав без второго [Scaffold] (13.1).
class TeamScreen extends ConsumerWidget {
  const TeamScreen({super.key, required this.projectId});

  final String projectId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    assert(projectId.isNotEmpty);
    final l10n = AppLocalizations.of(context)!;
    final asyncTeam = ref.watch(teamProvider(projectId));

    if (asyncTeam.hasError) {
      return DataLoadErrorMessage(
        title: l10n.dataLoadError,
        actionLabel: l10n.retry,
        onAction: () => ref.invalidate(teamProvider(projectId)),
      );
    }

    if (asyncTeam.isLoading || !asyncTeam.hasValue) {
      return const Center(child: CircularProgressIndicator());
    }

    final team = asyncTeam.requireValue;

    Future<void> onRefresh() async {
      ref.invalidate(teamProvider(projectId));
      try {
        await ref.read(teamProvider(projectId).future);
      } on Exception {
        // Состояние ошибки уже в asyncTeam; RefreshIndicator завершится.
      }
    }

    final agents = team.agents;
    final itemCount = agents.isEmpty ? 1 : agents.length;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
          child: _TeamHeaderBlock(team: team),
        ),
        Expanded(
          child: RefreshIndicator(
            onRefresh: onRefresh,
            child: ListView.builder(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              itemCount: itemCount,
              itemBuilder: (context, index) {
                if (agents.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 24),
                    child: Text(
                      l10n.teamEmptyAgents,
                      style: Theme.of(context).textTheme.bodyLarge,
                      textAlign: TextAlign.center,
                    ),
                  );
                }
                final agent = agents[index];
                return Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: _AgentRowTile(agent: agent),
                );
              },
            ),
          ),
        ),
      ],
    );
  }
}

class _TeamHeaderBlock extends StatelessWidget {
  const _TeamHeaderBlock({required this.team});

  final TeamModel team;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          team.name,
          style: theme.textTheme.titleLarge,
        ),
        const SizedBox(height: 4),
        Text(
          team.type,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }
}

class _AgentRowTile extends StatelessWidget {
  const _AgentRowTile({required this.agent});

  final AgentModel agent;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final roleText = agentRoleLabel(l10n, agent.role);
    final prompt = _optionalLine(agent.promptName);
    final backend = _optionalLine(agent.codeBackend);

    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        _agentDisplayName(l10n, agent.name),
                        style: theme.textTheme.titleMedium,
                      ),
                      const SizedBox(height: 4),
                      Text(
                        roleText,
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        _agentModelLine(l10n, agent.model),
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                      if (prompt != null) ...[
                        const SizedBox(height: 4),
                        Text(
                          prompt,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                      if (backend != null) ...[
                        const SizedBox(height: 4),
                        Text(
                          backend,
                          style: theme.textTheme.bodySmall?.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                const SizedBox(width: 8),
                Semantics(
                  label: agent.isActive
                      ? l10n.teamAgentActive
                      : l10n.teamAgentInactive,
                  child: Chip(
                    avatar: Icon(
                      agent.isActive ? Icons.check_circle : Icons.pause_circle,
                      size: 18,
                      color: agent.isActive
                          ? scheme.primary
                          : scheme.onSurfaceVariant,
                    ),
                    label: Text(
                      agent.isActive
                          ? l10n.teamAgentActive
                          : l10n.teamAgentInactive,
                    ),
                    visualDensity: VisualDensity.compact,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
