import 'package:flutter/material.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
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

/// Карточка строки агента команды: роль, модель, опционально промпт и code_backend, статус (13.2).
class AgentCard extends StatelessWidget {
  const AgentCard({
    super.key,
    required this.agent,
    this.onTap,
  });

  final AgentModel agent;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final roleText = agentRoleLabel(l10n, agent.role);
    final prompt = _optionalLine(agent.promptName);
    final backend = _optionalLine(agent.codeBackend);

    final padding = Padding(
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
    );

    final cardChild = onTap != null
        ? InkWell(
            onTap: onTap,
            child: padding,
          )
        : padding;

    final card = Card(
      margin: EdgeInsets.zero,
      child: cardChild,
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
