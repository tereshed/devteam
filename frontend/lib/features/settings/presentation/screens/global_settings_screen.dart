import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';
import 'package:frontend/features/settings/presentation/widgets/assistant_prompt_editor.dart';
import 'package:frontend/features/settings/domain/global_settings_backend_gate.dart';
import 'package:frontend/features/settings/presentation/widgets/claude_code_auth_section.dart';
import 'package:frontend/features/settings/presentation/widgets/llm_providers_section.dart';
import 'package:frontend/features/webhooks/presentation/widgets/webhooks_list_section.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Sprint 15.30/15.31 — глобальные настройки получили вкладки:
///   - LLM-провайдеры
///   - Claude Code (OAuth-подписка)
///   - PolyMaths (legacy 13.5 — заглушка про user API keys).
///   - Webhooks
class GlobalSettingsScreen extends StatelessWidget {
  const GlobalSettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'globalSettingsScreen',
    );
    final appL10n = AppLocalizations.of(context)!;
    return DefaultTabController(
      length: 5,
      child: Scaffold(
        appBar: AppBar(
          title: Text(l10n.globalSettingsScreenTitle),
          actions: const [LogoutButton()],
          bottom: TabBar(
            tabs: [
              Tab(text: l10n.globalSettingsTabLLMProviders),
              Tab(text: l10n.globalSettingsTabClaudeCode),
              Tab(text: l10n.globalSettingsTabDevTeam),
              Tab(text: appL10n.assistantPromptUserTabTitle),
              Tab(text: appL10n.webhooksTitle),
            ],
          ),
        ),
        body: SafeArea(
          child: AdaptiveContainer(
            child: const TabBarView(
              children: [
                _LLMProvidersTab(),
                _ClaudeCodeTab(),
                _PolyMathsLegacyTab(),
                _AssistantPromptTab(),
                _WebhooksTab(),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _WebhooksTab extends StatelessWidget {
  const _WebhooksTab();

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: Spacing.cardPadding(context),
      child: const WebhooksListSection(),
    );
  }
}

class _LLMProvidersTab extends StatelessWidget {
  const _LLMProvidersTab();

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: Spacing.cardPadding(context),
      child: const LLMProvidersSection(),
    );
  }
}

class _ClaudeCodeTab extends StatelessWidget {
  const _ClaudeCodeTab();

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: Spacing.cardPadding(context),
      child: const ClaudeCodeAuthSection(),
    );
  }
}

class _PolyMathsLegacyTab extends StatelessWidget {
  const _PolyMathsLegacyTab();

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'globalSettingsLegacyTab',
    );
    final theme = Theme.of(context);
    final muted = theme.colorScheme.onSurface.withValues(alpha: 0.72);

    return SingleChildScrollView(
      padding: Spacing.cardPadding(context),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          SizedBox(height: Spacing.medium(context)),
          Icon(
            Icons.cloud_off_outlined,
            size: 48,
            color: theme.colorScheme.primary.withValues(alpha: 0.85),
          ),
          SizedBox(height: Spacing.medium(context)),
          Text(l10n.globalSettingsStubIntro, style: theme.textTheme.bodyLarge),
          SizedBox(height: Spacing.large(context)),
          Text(
            l10n.globalSettingsBlockedByLabel,
            style: theme.textTheme.titleSmall,
          ),
          SizedBox(height: Spacing.small(context)),
          SelectableText(
            globalSettingsBackendBlockerDocsPath,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontFamily: 'monospace',
              color: muted,
            ),
          ),
          SizedBox(height: Spacing.xLarge(context)),
          Text(
            l10n.globalSettingsStubApiKeysNote,
            style: theme.textTheme.bodyMedium?.copyWith(color: muted),
          ),
          SizedBox(height: Spacing.small(context)),
          OutlinedButton.icon(
            onPressed: () {
              context.go(AppRoutePaths.profileApiKeys);
            },
            icon: const Icon(Icons.vpn_key_outlined),
            label: Text(l10n.globalSettingsOpenDevTeamApiKeys),
          ),
        ],
      ),
    );
  }
}

/// Вкладка «Ассистент» — редактор user-level промпта ассистента (GET /me/assistant,
/// PUT /me/agents/{id}). Промпт наследуется копией в новые проекты при создании.
class _AssistantPromptTab extends ConsumerWidget {
  const _AssistantPromptTab();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final async = ref.watch(myAssistantProvider);
    return async.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (e, _) => Center(child: Text(l10n.assistantPromptLoadError)),
      data: (agent) => SingleChildScrollView(
        padding: Spacing.cardPadding(context),
        child: AssistantPromptEditor(
          heading: l10n.assistantPromptUserHeading,
          hint: l10n.assistantPromptUserHint,
          initialValue: agent.systemPrompt ?? '',
          onSave: (value) async {
            await ref
                .read(myAgentsRepositoryProvider)
                .update(agent.id, systemPrompt: value);
            ref.invalidate(myAssistantProvider);
          },
        ),
      ),
    );
  }
}
