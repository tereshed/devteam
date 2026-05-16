import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/shared/widgets/integration_provider_card.dart';

/// Заглушка экрана LLM-интеграций (этап 1 ui_refactoring).
///
/// Реальный экран приходит в этапе 2 (см. `dashboard-redesign-plan.md` §5 Этап 2).
class LlmIntegrationsScreen extends StatelessWidget {
  const LlmIntegrationsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'LlmIntegrationsScreen',
    );
    final providers = <(String, String)>[
      (l10n.llmProviderClaudeCode, l10n.llmProviderClaudeCodeSubtitle),
      (l10n.llmProviderAnthropic, l10n.llmProviderAnthropicSubtitle),
      (l10n.llmProviderOpenAi, l10n.llmProviderOpenAiSubtitle),
      (l10n.llmProviderOpenRouter, l10n.llmProviderOpenRouterSubtitle),
      (l10n.llmProviderDeepSeek, l10n.llmProviderDeepSeekSubtitle),
      (l10n.llmProviderZhipu, l10n.llmProviderZhipuSubtitle),
    ];

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.integrationsLlmTitle,
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(height: 4),
          Text(
            l10n.integrationsLlmComingSoon,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 24),
          LayoutBuilder(
            builder: (context, constraints) {
              const minCardWidth = 280.0;
              final cols = (constraints.maxWidth / minCardWidth)
                  .floor()
                  .clamp(1, 4);
              const spacing = 16.0;
              final cardWidth =
                  (constraints.maxWidth - spacing * (cols - 1)) / cols;
              return Wrap(
                spacing: spacing,
                runSpacing: spacing,
                children: [
                  for (final (name, subtitle) in providers)
                    SizedBox(
                      width: cardWidth,
                      child: IntegrationProviderCard.disabled(
                        logo: Icon(
                          Icons.smart_toy_outlined,
                          size: 28,
                          color: Theme.of(context).colorScheme.primary,
                        ),
                        title: name,
                        subtitle: subtitle,
                        statusLabel: l10n.integrationsComingSoonChip,
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
