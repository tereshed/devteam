import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/shared/widgets/integration_action.dart';
import 'package:frontend/shared/widgets/integration_provider_card.dart';
import 'package:frontend/shared/widgets/integration_status.dart';

/// Фабрики `IntegrationProviderCard` для каждого LLM-провайдера на экране
/// LLM Integrations. Делает текущее визуальное состояние одной утиной типизации —
/// `LlmProviderConnection` → готовая карточка с правильными chip/CTA.
///
/// См. dashboard-redesign §4a.3 (DRY) — здесь нет custom-virsions карточки,
/// только конфигурация общего `IntegrationProviderCard`.

/// Конфигурация одного провайдера: brand-meta из l10n.
class _ProviderBrand {
  const _ProviderBrand({
    required this.title,
    required this.subtitle,
    required this.icon,
  });

  final String title;
  final String subtitle;
  final IconData icon;
}

_ProviderBrand _brandFor(
  AppLocalizations l10n,
  LlmIntegrationProvider provider,
) {
  switch (provider) {
    case LlmIntegrationProvider.claudeCodeOAuth:
      return _ProviderBrand(
        title: l10n.llmProviderClaudeCode,
        subtitle: l10n.llmProviderClaudeCodeSubtitle,
        icon: Icons.workspace_premium_outlined,
      );
    case LlmIntegrationProvider.anthropic:
      return _ProviderBrand(
        title: l10n.llmProviderAnthropic,
        subtitle: l10n.llmProviderAnthropicSubtitle,
        icon: Icons.psychology_outlined,
      );
    case LlmIntegrationProvider.openai:
      return _ProviderBrand(
        title: l10n.llmProviderOpenAi,
        subtitle: l10n.llmProviderOpenAiSubtitle,
        icon: Icons.smart_toy_outlined,
      );
    case LlmIntegrationProvider.openrouter:
      return _ProviderBrand(
        title: l10n.llmProviderOpenRouter,
        subtitle: l10n.llmProviderOpenRouterSubtitle,
        icon: Icons.alt_route_outlined,
      );
    case LlmIntegrationProvider.deepseek:
      return _ProviderBrand(
        title: l10n.llmProviderDeepSeek,
        subtitle: l10n.llmProviderDeepSeekSubtitle,
        icon: Icons.water_drop_outlined,
      );
    case LlmIntegrationProvider.zhipu:
      return _ProviderBrand(
        title: l10n.llmProviderZhipu,
        subtitle: l10n.llmProviderZhipuSubtitle,
        icon: Icons.translate_outlined,
      );
    case LlmIntegrationProvider.gemini:
      return _ProviderBrand(
        title: 'Gemini',
        subtitle: l10n.llmProviderOpenAiSubtitle,
        icon: Icons.diamond_outlined,
      );
    case LlmIntegrationProvider.qwen:
      return _ProviderBrand(
        title: 'Qwen',
        subtitle: l10n.llmProviderOpenAiSubtitle,
        icon: Icons.bubble_chart_outlined,
      );
  }
}

/// Сопоставление [LlmProviderConnectionStatus] с UI-enum [IntegrationStatus].
IntegrationStatus _toIntegrationStatus(LlmProviderConnectionStatus s) {
  switch (s) {
    case LlmProviderConnectionStatus.connected:
      return IntegrationStatus.connected;
    case LlmProviderConnectionStatus.disconnected:
      return IntegrationStatus.disconnected;
    case LlmProviderConnectionStatus.error:
      return IntegrationStatus.error;
    case LlmProviderConnectionStatus.pending:
      return IntegrationStatus.pending;
  }
}

/// Возвращает локализованный текст под chip'ом для error/pending.
///
/// §4a.5: cancel (`user_cancelled`) / `invalid_state` / `provider_unreachable`.
String? _statusDetailFor(
  BuildContext context,
  AppLocalizations l10n,
  LlmProviderConnection conn,
) {
  if (conn.status == LlmProviderConnectionStatus.pending) {
    return l10n.integrationsLlmReasonPending;
  }
  if (conn.status != LlmProviderConnectionStatus.error) {
    if (conn.maskedPreview != null && conn.maskedPreview!.isNotEmpty) {
      return conn.maskedPreview;
    }
    return null;
  }
  switch (conn.reason) {
    case 'user_cancelled':
    case 'access_denied':
      return l10n.integrationsLlmReasonUserCancelled;
    case 'expired_token':
    case 'invalid_grant':
    case 'invalid_state':
      return l10n.integrationsLlmReasonExpired;
    case 'provider_unreachable':
    case 'internal_error':
      return l10n.integrationsLlmReasonProviderUnreachable;
    default:
      return l10n.integrationsLlmReasonUnknown(conn.reason ?? '');
  }
}

/// Собирает единый `IntegrationProviderCard` для провайдера.
///
/// `onConnect` / `onDisconnect` / `onReplace` — vois* callbacks из экрана.
/// Любой может быть null → соответствующая кнопка не отрендерится.
IntegrationProviderCard llmProviderCard(
  BuildContext context, {
  required LlmIntegrationProvider provider,
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) {
  final l10n = requireAppLocalizations(context, where: 'llmProviderCard');
  final brand = _brandFor(l10n, provider);
  final status = _toIntegrationStatus(connection.status);
  final actions = <IntegrationAction>[];
  if (connection.status == LlmProviderConnectionStatus.connected) {
    if (onReplace != null) {
      actions.add(IntegrationAction(
        label: l10n.integrationsLlmReplaceCta,
        onPressed: onReplace,
        isBusy: busy,
      ));
    }
    if (onDisconnect != null) {
      actions.add(IntegrationAction(
        label: l10n.integrationsLlmDisconnectCta,
        style: IntegrationActionStyle.destructive,
        onPressed: onDisconnect,
        isBusy: busy,
      ));
    }
  } else {
    if (onConnect != null) {
      actions.add(IntegrationAction(
        label: connection.status == LlmProviderConnectionStatus.error
            ? l10n.integrationsLlmRetry
            : l10n.integrationsLlmConnectCta,
        style: IntegrationActionStyle.primary,
        onPressed: onConnect,
        isBusy: busy || connection.status == LlmProviderConnectionStatus.pending,
      ));
    }
  }
  return IntegrationProviderCard(
    logo: Icon(
      brand.icon,
      size: 28,
      color: Theme.of(context).colorScheme.primary,
    ),
    title: brand.title,
    subtitle: brand.subtitle,
    status: status,
    statusDetail: _statusDetailFor(context, l10n, connection),
    actions: actions,
  );
}

/// Фабрики per-provider — удобная точка для тестов §4a.3 (one-file source of truth).
IntegrationProviderCard claudeCodeCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.claudeCodeOAuth,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      busy: busy,
    );

IntegrationProviderCard anthropicCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.anthropic,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      onReplace: onReplace,
      busy: busy,
    );

IntegrationProviderCard openAiCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.openai,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      onReplace: onReplace,
      busy: busy,
    );

IntegrationProviderCard openRouterCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.openrouter,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      onReplace: onReplace,
      busy: busy,
    );

IntegrationProviderCard deepseekCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.deepseek,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      onReplace: onReplace,
      busy: busy,
    );

IntegrationProviderCard zhipuCard(
  BuildContext context, {
  required LlmProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onReplace,
  bool busy = false,
}) =>
    llmProviderCard(
      context,
      provider: LlmIntegrationProvider.zhipu,
      connection: connection,
      onConnect: onConnect,
      onDisconnect: onDisconnect,
      onReplace: onReplace,
      busy: busy,
    );
