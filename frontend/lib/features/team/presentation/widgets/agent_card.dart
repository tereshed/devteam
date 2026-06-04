import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/l10n/app_localizations.dart';

String _agentDisplayName(AppLocalizations l10n, String name) {
  final t = name.trim();
  return t.isEmpty ? l10n.teamAgentNameUnset : t;
}

String? _optionalLine(String? value) {
  final v = value?.trim();
  if (v == null || v.isEmpty) {
    return null;
  }
  return v;
}

IconData _roleIcon(String role) => switch (role) {
      'planner' => Icons.architecture,
      'decomposer' => Icons.account_tree_outlined,
      'developer' => Icons.code,
      'reviewer' => Icons.rate_review_outlined,
      'tester' => Icons.bug_report_outlined,
      'merger' => Icons.merge_type,
      'router' => Icons.call_split,
      'orchestrator' => Icons.hub_outlined,
      'assistant' => Icons.smart_toy_outlined,
      _ => Icons.android,
    };

/// Плотная строка агента команды: роль-иконка, имя, роль, модель/бекенд,
/// провайдер и компактный статус (13.2, redesign).
class AgentCard extends StatelessWidget {
  const AgentCard({
    super.key,
    required this.agent,
    this.onTap,
    this.onDelete,
  });

  final AgentModel agent;
  final VoidCallback? onTap;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'agentCard');
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    // У большинства ролей есть и модель, и бекенд — показываем всё, что задано.
    final backend = _optionalLine(agent.codeBackend);
    final model = _optionalLine(agent.model) ?? (backend == null ? l10n.teamAgentModelUnset : null);
    final provider = _optionalLine(agent.providerKind);
    final activeColor =
        agent.isActive ? const Color(0xFF3FB950) : scheme.onSurfaceVariant;

    final content = Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          CircleAvatar(
            radius: 16,
            backgroundColor: scheme.surfaceContainerHighest,
            child: Icon(_roleIcon(agent.role),
                size: 17, color: scheme.onSurfaceVariant),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        _agentDisplayName(l10n, agent.name),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.titleSmall
                            ?.copyWith(fontWeight: FontWeight.w600),
                      ),
                    ),
                    const SizedBox(width: 8),
                    _RolePill(label: agentRoleLabel(l10n, agent.role)),
                  ],
                ),
                const SizedBox(height: 6),
                Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  children: [
                    if (model != null) _MonoChip(text: model),
                    if (backend != null) _MonoChip(text: backend),
                    if (provider != null) _MonoChip(text: provider),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          Semantics(
            label: agent.isActive ? l10n.teamAgentActive : l10n.teamAgentInactive,
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: 8,
                  height: 8,
                  decoration:
                      BoxDecoration(color: activeColor, shape: BoxShape.circle),
                ),
                const SizedBox(width: 6),
                Text(
                  agent.isActive ? l10n.teamAgentActive : l10n.teamAgentInactive,
                  style: theme.textTheme.labelSmall?.copyWith(color: activeColor),
                ),
              ],
            ),
          ),
          if (onDelete != null) ...[
            const SizedBox(width: 4),
            IconButton(
              icon: const Icon(Icons.delete_outline, size: 20),
              color: scheme.error,
              tooltip: l10n.delete,
              onPressed: onDelete,
            ),
          ],
        ],
      ),
    );

    final card = Card(
      margin: EdgeInsets.zero,
      clipBehavior: Clip.antiAlias,
      child: onTap != null
          ? InkWell(onTap: onTap, child: content)
          : content,
    );

    if (onTap != null) {
      return Semantics(
        container: true,
        button: true,
        label: _agentDisplayName(l10n, agent.name),
        child: card,
      );
    }
    return card;
  }
}

class _RolePill extends StatelessWidget {
  const _RolePill({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: scheme.secondaryContainer,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(
              color: scheme.onSecondaryContainer,
              fontWeight: FontWeight.w600,
            ),
      ),
    );
  }
}

class _MonoChip extends StatelessWidget {
  const _MonoChip({required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest.withValues(alpha: 0.6),
        borderRadius: BorderRadius.circular(5),
      ),
      child: Text(
        text,
        style: const TextStyle(
          fontFamily: 'monospace',
          fontSize: 11,
        ).copyWith(color: scheme.onSurfaceVariant),
      ),
    );
  }
}
