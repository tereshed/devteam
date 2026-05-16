import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/integrations/llm/presentation/widgets/connect_api_key_dialog.dart';
import 'package:frontend/features/integrations/llm/presentation/widgets/connect_claude_code_dialog.dart';
import 'package:frontend/features/integrations/llm/presentation/widgets/llm_provider_cards.dart';

/// Экран `/integrations/llm` — управление LLM-провайдерами для текущего юзера.
///
/// SSOT поведения:
///   * dashboard-redesign §4a.3 — universal `IntegrationProviderCard`.
///   * dashboard-redesign §4a.4 — realtime через `IntegrationConnectionChanged`.
///   * dashboard-redesign §4a.5 — error UI states (cancel / invalid_state / network).
class LlmIntegrationsScreen extends ConsumerStatefulWidget {
  const LlmIntegrationsScreen({super.key});

  @override
  ConsumerState<LlmIntegrationsScreen> createState() =>
      _LlmIntegrationsScreenState();
}

class _LlmIntegrationsScreenState extends ConsumerState<LlmIntegrationsScreen> {
  // Список провайдеров, которые отображаются на экране в фиксированном порядке.
  static const _displayOrder = <LlmIntegrationProvider>[
    LlmIntegrationProvider.claudeCodeOAuth,
    LlmIntegrationProvider.anthropic,
    LlmIntegrationProvider.openai,
    LlmIntegrationProvider.openrouter,
    LlmIntegrationProvider.deepseek,
    LlmIntegrationProvider.zhipu,
  ];

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      // §4a.4: ровно один REST-fetch на open; дальше — события из WS.
      ref.read(llmIntegrationsControllerProvider).refresh();
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'LlmIntegrationsScreen',
    );
    // Подписываемся на контроллер как ChangeNotifier — обновляемся на каждое уведомление.
    final controller = ref.watch(llmIntegrationsControllerProvider);
    return AnimatedBuilder(
      animation: controller,
      builder: (context, _) {
        final state = controller.state;
        final connected = _displayOrder
            .where((p) =>
                state.connections[p]?.status ==
                LlmProviderConnectionStatus.connected)
            .toList(growable: false);
        final available = _displayOrder
            .where((p) =>
                state.connections[p]?.status !=
                LlmProviderConnectionStatus.connected)
            .toList(growable: false);

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
                l10n.integrationsLlmStage2Subtitle,
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
                    message: l10n.integrationsLlmLoadFailed(
                      state.errorMessage!,
                    ),
                    onRetry: () => ref
                        .read(llmIntegrationsControllerProvider)
                        .refresh(),
                  ),
                  const SizedBox(height: 16),
                ],
                _Section(
                  title: l10n.integrationsLlmSectionConnected,
                  emptyHint: connected.isEmpty
                      ? l10n.integrationsLlmEmptyAvailable
                      : null,
                  children: [
                    for (final p in connected)
                      _wrapCard(_buildCard(context, p, state)),
                  ],
                ),
                const SizedBox(height: 24),
                _Section(
                  title: l10n.integrationsLlmSectionAvailable,
                  emptyHint: available.isEmpty
                      ? l10n.integrationsLlmEmptyAvailable
                      : null,
                  children: [
                    for (final p in available)
                      _wrapCard(_buildCard(context, p, state)),
                  ],
                ),
              ],
            ],
          ),
        );
      },
    );
  }

  Widget _buildCard(
    BuildContext context,
    LlmIntegrationProvider provider,
    LlmIntegrationsState state,
  ) {
    final conn = state.connections[provider] ??
        LlmProviderConnection(
          provider: provider,
          status: LlmProviderConnectionStatus.disconnected,
        );
    void onConnect() => _onConnect(context, provider, conn);
    void onDisconnect() => _onDisconnect(provider);
    void onReplace() => _onConnect(context, provider, conn);
    return llmProviderCard(
      context,
      provider: provider,
      connection: conn,
      onConnect: provider == LlmIntegrationProvider.claudeCodeOAuth
          ? (conn.status == LlmProviderConnectionStatus.connected
              ? null
              : onConnect)
          : (conn.status == LlmProviderConnectionStatus.connected
              ? null
              : onConnect),
      onDisconnect:
          conn.status == LlmProviderConnectionStatus.connected ? onDisconnect : null,
      onReplace: provider == LlmIntegrationProvider.claudeCodeOAuth
          ? null
          : (conn.status == LlmProviderConnectionStatus.connected
              ? onReplace
              : null),
    );
  }

  Future<void> _onConnect(
    BuildContext context,
    LlmIntegrationProvider provider,
    LlmProviderConnection current,
  ) async {
    if (provider == LlmIntegrationProvider.claudeCodeOAuth) {
      await showConnectClaudeCodeDialog(context, ref);
    } else {
      await showConnectApiKeyDialog(context, ref, provider: provider);
    }
  }

  Future<void> _onDisconnect(LlmIntegrationProvider provider) async {
    final repo = ref.read(llmIntegrationsRepositoryProvider);
    final controller = ref.read(llmIntegrationsControllerProvider);
    try {
      if (provider == LlmIntegrationProvider.claudeCodeOAuth) {
        await repo.revokeClaudeCodeOAuth();
      } else {
        await repo.clearApiKey(provider: provider);
      }
      controller.applyLocal(
        LlmProviderConnection(
          provider: provider,
          status: LlmProviderConnectionStatus.disconnected,
        ),
      );
    } catch (_) {
      // Ошибки REST покажет следующее обновление состояния (через refresh).
      await controller.refresh();
    }
  }

  Widget _wrapCard(Widget card) =>
      SizedBox(width: 320, child: card);
}

class _Section extends StatelessWidget {
  const _Section({
    required this.title,
    required this.children,
    this.emptyHint,
  });

  final String title;
  final List<Widget> children;
  final String? emptyHint;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.w600,
              ),
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
          Wrap(
            spacing: 16,
            runSpacing: 16,
            children: children,
          ),
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
            child: Text(l10n.integrationsLlmRetry),
          ),
        ],
      ),
    );
  }
}
