import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/onboarding/data/onboarding_providers.dart';
import 'package:frontend/features/onboarding/presentation/widgets/onboarding_banner.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/presentation/widgets/agent_edit_dialog.dart';

class ProjectOnboardingBanner extends ConsumerWidget {
  const ProjectOnboardingBanner({super.key, required this.projectId});

  final String projectId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(projectOnboardingStateProvider(projectId));

    if (state.loading || !state.needsAgentSetup) {
      return const SizedBox.shrink();
    }

    final l10n = requireAppLocalizations(
      context,
      where: 'ProjectOnboardingBanner',
    );

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
      child: OnboardingBanner(
        icon: Icons.settings_suggest,
        message: l10n.onboardingConfigureProjectAgents,
        actionLabel: l10n.onboardingGoToTeam,
        onAction: () => _openFirstUnconfiguredAgent(context, ref),
      ),
    );
  }

  void _openFirstUnconfiguredAgent(BuildContext context, WidgetRef ref) {
    final asyncTeam = ref.read(teamProvider(projectId));
    if (!asyncTeam.hasValue) {
      return;
    }
    final agents = asyncTeam.requireValue.agents;

    bool isUnconfigured(AgentModel a) =>
        a.role == 'router' &&
        (a.model == null ||
            a.model!.isEmpty ||
            a.providerKind == null ||
            a.providerKind!.isEmpty);

    final target = agents.where(isUnconfigured).firstOrNull;
    if (target == null) {
      return;
    }

    showAgentEditDialog(context, projectId: projectId, agent: target);
  }
}
