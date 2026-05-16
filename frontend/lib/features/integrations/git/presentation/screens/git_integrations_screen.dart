import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/features/integrations/git/presentation/widgets/connect_gitlab_host_dialog.dart';
import 'package:frontend/features/integrations/git/presentation/widgets/git_provider_cards.dart';
import 'package:url_launcher/url_launcher.dart';

/// Экран `/integrations/git` — управление GitHub/GitLab подключениями.
///
/// SSOT поведения:
///   * dashboard-redesign §4a.3 — universal `IntegrationProviderCard`.
///   * dashboard-redesign §4a.4 — realtime через `IntegrationConnectionChanged`.
///   * dashboard-redesign §4a.5 — error UI states (cancel / invalid_state / network /
///     invalid_host).
///   * dashboard-redesign §4a.1 — `remote_revoke_failed` подсветка после revoke.
class GitIntegrationsScreen extends ConsumerWidget {
  const GitIntegrationsScreen({super.key});

  static const _displayOrder = <GitIntegrationProvider>[
    GitIntegrationProvider.github,
    GitIntegrationProvider.gitlab,
  ];

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(
      context,
      where: 'GitIntegrationsScreen',
    );
    final state = ref.watch(gitIntegrationsControllerProvider);
    final connected = _displayOrder
        .where(
          (p) =>
              state.connections[p]?.status ==
              GitProviderConnectionStatus.connected,
        )
        .toList(growable: false);
    final available = _displayOrder
        .where(
          (p) =>
              state.connections[p]?.status !=
              GitProviderConnectionStatus.connected,
        )
        .toList(growable: false);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            l10n.integrationsGitTitle,
            style: Theme.of(
              context,
            ).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w700),
          ),
          const SizedBox(height: 4),
          Text(
            l10n.integrationsGitStage3Subtitle,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 24),
          if (state.isLoading && state.connections.isEmpty)
            const Center(
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 32),
                child: CircularProgressIndicator(),
              ),
            )
          else ...[
            if (state.errorMessage != null) ...[
              _ErrorBanner(
                message: l10n.integrationsGitLoadFailed(state.errorMessage!),
                onRetry: () => ref
                    .read(gitIntegrationsControllerProvider.notifier)
                    .refresh(),
              ),
              const SizedBox(height: 16),
            ],
            LayoutBuilder(
              builder: (context, constraints) {
                // Mobile (<600) — one column; wider — two.
                final cols = constraints.maxWidth < 600 ? 1 : 2;
                const spacing = 16.0;
                final cardWidth = cols == 1
                    ? constraints.maxWidth
                    : (constraints.maxWidth - spacing) / 2;
                return _SectionsLayout(
                  cardWidth: cardWidth,
                  spacing: spacing,
                  connected: connected,
                  available: available,
                  state: state,
                  buildCard: (provider) =>
                      _buildCard(context, ref, provider, state),
                );
              },
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildCard(
    BuildContext context,
    WidgetRef ref,
    GitIntegrationProvider provider,
    GitIntegrationsState state,
  ) {
    final conn =
        state.connections[provider] ??
        GitProviderConnection(
          provider: provider,
          status: GitProviderConnectionStatus.disconnected,
        );
    final isConnected = conn.status == GitProviderConnectionStatus.connected;
    void onConnect() => _onConnect(context, ref, provider);
    void onDisconnect() => _onDisconnect(ref, provider);
    void onConnectSelfHosted() => _onConnectSelfHosted(context, ref);

    switch (provider) {
      case GitIntegrationProvider.github:
        return githubCard(
          context,
          connection: conn,
          onConnect: isConnected ? null : onConnect,
          onDisconnect: isConnected ? onDisconnect : null,
        );
      case GitIntegrationProvider.gitlab:
        return gitlabCard(
          context,
          connection: conn,
          onConnect: isConnected ? null : onConnect,
          onDisconnect: isConnected ? onDisconnect : null,
          onConnectSelfHosted: isConnected ? null : onConnectSelfHosted,
        );
    }
  }

  /// Конструирует URL коллбэка, который мы заявляем провайдеру при `init`.
  ///
  /// Совпадает с `redirect_uri`, который зарегистрирован в OAuth App
  /// (см. `oauth-setup-guide.md` §5). Бэк проверит match.
  String _redirectUri(WidgetRef ref, GitIntegrationProvider provider) {
    final base = ref.read(dioClientProvider).options.baseUrl;
    final trimmed = base.endsWith('/')
        ? base.substring(0, base.length - 1)
        : base;
    return '$trimmed/integrations/${provider.jsonValue}/auth/callback';
  }

  Future<void> _onConnect(
    BuildContext context,
    WidgetRef ref,
    GitIntegrationProvider provider,
  ) async {
    final controller = ref.read(gitIntegrationsControllerProvider.notifier);
    final String authorizeUrl;
    try {
      authorizeUrl = await controller.initConnection(
        provider,
        redirectUri: _redirectUri(ref, provider),
      );
    } catch (_) {
      // Контроллер сам обновил state в error — UI просто прерывает flow.
      return;
    }
    final uri = Uri.tryParse(authorizeUrl);
    if (uri == null) {
      controller.rollbackToDisconnected(provider);
      return;
    }
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (ok) {
      return;
    }
    controller.rollbackToDisconnected(provider);
    if (!context.mounted) {
      return;
    }
    final l10n = requireAppLocalizations(
      context,
      where: 'GitIntegrationsScreen.onConnect',
    );
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(l10n.integrationsGitBrowserOpenFailed(uri.toString())),
      ),
    );
  }

  Future<void> _onConnectSelfHosted(BuildContext context, WidgetRef ref) async {
    final redirectUri = _redirectUri(ref, GitIntegrationProvider.gitlab);
    // showAndLaunchConnectGitlabHost сам обновит локальный стейт в pending,
    // попытается открыть authorize_url в браузере и откатит pending при провале.
    await showAndLaunchConnectGitlabHost(
      context,
      ref,
      redirectUri: redirectUri,
    );
  }

  Future<void> _onDisconnect(
    WidgetRef ref,
    GitIntegrationProvider provider,
  ) async {
    await ref
        .read(gitIntegrationsControllerProvider.notifier)
        .disconnect(provider);
  }
}

class _SectionsLayout extends StatelessWidget {
  const _SectionsLayout({
    required this.cardWidth,
    required this.spacing,
    required this.connected,
    required this.available,
    required this.state,
    required this.buildCard,
  });

  final double cardWidth;
  final double spacing;
  final List<GitIntegrationProvider> connected;
  final List<GitIntegrationProvider> available;
  final GitIntegrationsState state;
  final Widget Function(GitIntegrationProvider) buildCard;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_SectionsLayout');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _Section(
          title: l10n.integrationsGitSectionConnected,
          emptyHint: connected.isEmpty
              ? l10n.integrationsGitEmptyAvailable
              : null,
          spacing: spacing,
          children: [
            for (final p in connected)
              SizedBox(width: cardWidth, child: buildCard(p)),
          ],
        ),
        const SizedBox(height: 24),
        _Section(
          title: l10n.integrationsGitSectionAvailable,
          emptyHint: available.isEmpty
              ? l10n.integrationsGitEmptyAvailable
              : null,
          spacing: spacing,
          children: [
            for (final p in available)
              SizedBox(width: cardWidth, child: buildCard(p)),
          ],
        ),
      ],
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({
    required this.title,
    required this.children,
    required this.spacing,
    this.emptyHint,
  });

  final String title;
  final List<Widget> children;
  final String? emptyHint;
  final double spacing;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: Theme.of(
            context,
          ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 12),
        if (children.isEmpty && emptyHint != null)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 16),
            child: Text(
              emptyHint!,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          )
        else
          Wrap(spacing: spacing, runSpacing: spacing, children: children),
      ],
    );
  }
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final l10n = requireAppLocalizations(context, where: '_ErrorBanner');
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: scheme.errorContainer,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(Icons.error_outline, color: scheme.error),
          const SizedBox(width: 12),
          Expanded(
            child: Text(
              message,
              style: TextStyle(color: scheme.onErrorContainer),
            ),
          ),
          TextButton(
            onPressed: onRetry,
            child: Text(l10n.integrationsGitRetry),
          ),
        ],
      ),
    );
  }
}
