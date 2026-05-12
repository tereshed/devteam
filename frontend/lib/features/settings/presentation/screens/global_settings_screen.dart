import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/features/settings/domain/global_settings_backend_gate.dart';
import 'package:frontend/features/settings/presentation/widgets/claude_code_auth_section.dart';
import 'package:frontend/features/settings/presentation/widgets/llm_providers_section.dart';
import 'package:go_router/go_router.dart';

/// Sprint 15.30/15.31 — глобальные настройки получили вкладки:
///   - LLM-провайдеры
///   - Claude Code (OAuth-подписка)
///   - DevTeam (legacy 13.5 — заглушка про user API keys).
class GlobalSettingsScreen extends StatelessWidget {
  const GlobalSettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'globalSettingsScreen',
    );
    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          title: Text(l10n.globalSettingsScreenTitle),
          actions: const [LogoutButton()],
          bottom: TabBar(
            tabs: [
              Tab(text: l10n.globalSettingsTabLLMProviders),
              Tab(text: l10n.globalSettingsTabClaudeCode),
              Tab(text: l10n.globalSettingsTabDevTeam),
            ],
          ),
        ),
        body: SafeArea(
          child: AdaptiveContainer(
            child: const TabBarView(
              children: [
                _LLMProvidersTab(),
                _ClaudeCodeTab(),
                _DevTeamLegacyTab(),
              ],
            ),
          ),
        ),
      ),
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

class _DevTeamLegacyTab extends StatelessWidget {
  const _DevTeamLegacyTab();

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
