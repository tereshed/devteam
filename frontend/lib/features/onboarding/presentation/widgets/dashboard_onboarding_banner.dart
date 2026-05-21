import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/onboarding/data/onboarding_providers.dart';
import 'package:frontend/features/onboarding/presentation/widgets/onboarding_banner.dart';
import 'package:go_router/go_router.dart';

class DashboardOnboardingBanner extends ConsumerWidget {
  const DashboardOnboardingBanner({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(onboardingStateProvider);

    if (state.loading || !state.needsAssistantSetup) {
      return const SizedBox.shrink();
    }

    final l10n = requireAppLocalizations(
      context,
      where: 'DashboardOnboardingBanner',
    );

    if (!state.hasLlmProviders) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 16),
        child: OnboardingBanner(
          icon: Icons.power_settings_new,
          message: l10n.onboardingConnectLlmProvider,
          actionLabel: l10n.onboardingGoToSettings,
          onAction: () => context.go('/integrations/llm'),
        ),
      );
    }

    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: OnboardingBanner(
        icon: Icons.smart_toy_outlined,
        message: l10n.onboardingConfigureAssistant,
        actionLabel: l10n.onboardingGoToSettings,
        onAction: () => context.go('/settings'),
      ),
    );
  }
}
