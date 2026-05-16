import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/shared/widgets/integration_action.dart';
import 'package:frontend/shared/widgets/integration_provider_card.dart';
import 'package:frontend/shared/widgets/integration_status.dart';

/// Заглушка экрана Git-интеграций (этап 1 ui_refactoring).
///
/// Реальный экран приходит в этапе 3b (см. `dashboard-redesign-plan.md` §5 Этап 3).
class GitIntegrationsScreen extends StatelessWidget {
  const GitIntegrationsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'GitIntegrationsScreen',
    );

    final providers = <(String, String, IconData)>[
      (
        l10n.gitProviderGithub,
        l10n.integrationsGitGithubSubtitle,
        Icons.code,
      ),
      (
        l10n.gitProviderGitlab,
        l10n.integrationsGitGitlabSubtitle,
        Icons.fork_right,
      ),
    ];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.integrationsGitTitle,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(height: 4),
          Text(
            l10n.integrationsGitComingSoon,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 24),
          LayoutBuilder(
            builder: (context, constraints) {
              const minCardWidth = 320.0;
              final cols = (constraints.maxWidth / minCardWidth)
                  .floor()
                  .clamp(1, 2);
              const spacing = 16.0;
              final cardWidth =
                  (constraints.maxWidth - spacing * (cols - 1)) / cols;
              return Wrap(
                spacing: spacing,
                runSpacing: spacing,
                children: [
                  for (final (name, subtitle, icon) in providers)
                    SizedBox(
                      width: cardWidth,
                      child: IntegrationProviderCard(
                        logo: Icon(
                          icon,
                          size: 28,
                          color: Theme.of(context).colorScheme.primary,
                        ),
                        title: name,
                        subtitle: subtitle,
                        status: IntegrationStatus.disconnected,
                        actions: [
                          IntegrationAction(
                            label: l10n.integrationsGitConnectCta,
                            onPressed: () {
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(
                                  content: Text(l10n.integrationsGitComingSoon),
                                ),
                              );
                            },
                            style: IntegrationActionStyle.primary,
                            icon: Icons.link,
                          ),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}
