import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/features/settings/domain/global_settings_backend_gate.dart';
import 'package:go_router/go_router.dart';

/// Экран глобальных настроек LLM-провайдеров (13.5).
///
/// **Режим B:** backend для `GET`/`PATCH` пользовательских ключей ещё не готов —
/// показывается read-only заглушка (см. `docs/tasks/13.5-global-settings-screen.md`).
class GlobalSettingsScreen extends StatelessWidget {
  const GlobalSettingsScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'globalSettingsScreen',
    );
    final theme = Theme.of(context);
    final muted = theme.colorScheme.onSurface.withValues(alpha: 0.72);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.globalSettingsScreenTitle),
        actions: const [LogoutButton()],
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          child: Container(
            alignment: Alignment.topCenter,
            child: AdaptiveContainer(
              child: Padding(
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
                    Text(
                      l10n.globalSettingsStubIntro,
                      style: theme.textTheme.bodyLarge,
                    ),
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
              ),
            ),
          ),
        ),
      ),
    );
  }
}
