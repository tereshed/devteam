import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/shared/widgets/integration_action.dart';
import 'package:frontend/shared/widgets/integration_provider_card.dart';
import 'package:frontend/shared/widgets/integration_status.dart';

/// Фабрики `IntegrationProviderCard` для GitHub/GitLab на экране Git Integrations.
///
/// Зеркало `llm_provider_cards.dart` — общая карточка `IntegrationProviderCard`
/// (см. dashboard-redesign §4a.3) с конфигурацией бренда + actions.

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

_ProviderBrand _brandFor(AppLocalizations l10n, GitIntegrationProvider p) {
  switch (p) {
    case GitIntegrationProvider.github:
      return _ProviderBrand(
        title: l10n.gitProviderGithub,
        subtitle: l10n.integrationsGitGithubSubtitle,
        icon: Icons.code,
      );
    case GitIntegrationProvider.gitlab:
      return _ProviderBrand(
        title: l10n.gitProviderGitlab,
        subtitle: l10n.integrationsGitGitlabSubtitle,
        icon: Icons.fork_right,
      );
  }
}

IntegrationStatus _toIntegrationStatus(GitProviderConnectionStatus s) {
  switch (s) {
    case GitProviderConnectionStatus.connected:
      return IntegrationStatus.connected;
    case GitProviderConnectionStatus.disconnected:
      return IntegrationStatus.disconnected;
    case GitProviderConnectionStatus.error:
      return IntegrationStatus.error;
    case GitProviderConnectionStatus.pending:
      return IntegrationStatus.pending;
  }
}

/// Локализованный текст под chip'ом. §4a.5 + §4a.1 (remote_revoke_failed).
String? _statusDetailFor(AppLocalizations l10n, GitProviderConnection conn) {
  if (conn.remoteRevokeFailed) {
    return l10n.integrationsGitReasonRemoteRevokeFailed;
  }
  if (conn.status == GitProviderConnectionStatus.pending) {
    return l10n.integrationsGitReasonPending;
  }
  if (conn.status == GitProviderConnectionStatus.connected) {
    // Для подключённого GitLab показываем host (для BYO это критично).
    if (conn.host != null && conn.host!.isNotEmpty) {
      return l10n.integrationsGitConnectedHost(conn.host!);
    }
    if (conn.accountLogin != null && conn.accountLogin!.isNotEmpty) {
      return l10n.integrationsGitConnectedAccount(conn.accountLogin!);
    }
    return null;
  }
  if (conn.status != GitProviderConnectionStatus.error) {
    return null;
  }
  switch (conn.reason) {
    case 'user_cancelled':
    case 'access_denied':
      return l10n.integrationsGitReasonUserCancelled;
    case 'expired_token':
    case 'invalid_grant':
    case 'invalid_state':
      return l10n.integrationsGitReasonExpired;
    case 'provider_unreachable':
    case 'internal_error':
      return l10n.integrationsGitReasonProviderUnreachable;
    case 'invalid_host':
      return l10n.integrationsGitReasonInvalidHost;
    case 'oauth_not_configured':
      return l10n.integrationsGitReasonOauthNotConfigured;
    default:
      return l10n.integrationsGitReasonUnknown(conn.reason ?? '');
  }
}

/// Универсальная фабрика карточки для git-провайдера.
///
/// `onConnect` / `onDisconnect` / `onConnectSelfHosted` могут быть null —
/// тогда соответствующая кнопка не появится. `onConnectSelfHosted` имеет
/// смысл только для GitLab.
IntegrationProviderCard gitProviderCard(
  BuildContext context, {
  required GitIntegrationProvider provider,
  required GitProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onConnectSelfHosted,
  bool busy = false,
}) {
  final l10n = requireAppLocalizations(context, where: 'gitProviderCard');
  final brand = _brandFor(l10n, provider);
  final status = _toIntegrationStatus(connection.status);
  final actions = <IntegrationAction>[];
  if (connection.status == GitProviderConnectionStatus.connected) {
    if (onDisconnect != null) {
      actions.add(
        IntegrationAction(
          label: l10n.integrationsGitDisconnectCta,
          style: IntegrationActionStyle.destructive,
          onPressed: onDisconnect,
          isBusy: busy,
        ),
      );
    }
  } else {
    if (onConnect != null) {
      actions.add(
        IntegrationAction(
          label: connection.status == GitProviderConnectionStatus.error
              ? l10n.integrationsGitRetry
              : l10n.integrationsGitConnectCta,
          style: IntegrationActionStyle.primary,
          onPressed: onConnect,
          icon: Icons.link,
          isBusy:
              busy || connection.status == GitProviderConnectionStatus.pending,
        ),
      );
    }
    if (onConnectSelfHosted != null) {
      actions.add(
        IntegrationAction(
          label: l10n.integrationsGitConnectSelfHostedCta,
          style: IntegrationActionStyle.secondary,
          onPressed: onConnectSelfHosted,
          icon: Icons.dns_outlined,
          isBusy: busy,
        ),
      );
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
    statusDetail: _statusDetailFor(l10n, connection),
    actions: actions,
  );
}

IntegrationProviderCard githubCard(
  BuildContext context, {
  required GitProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  bool busy = false,
}) => gitProviderCard(
  context,
  provider: GitIntegrationProvider.github,
  connection: connection,
  onConnect: onConnect,
  onDisconnect: onDisconnect,
  busy: busy,
);

IntegrationProviderCard gitlabCard(
  BuildContext context, {
  required GitProviderConnection connection,
  VoidCallback? onConnect,
  VoidCallback? onDisconnect,
  VoidCallback? onConnectSelfHosted,
  bool busy = false,
}) => gitProviderCard(
  context,
  provider: GitIntegrationProvider.gitlab,
  connection: connection,
  onConnect: onConnect,
  onDisconnect: onDisconnect,
  onConnectSelfHosted: onConnectSelfHosted,
  busy: busy,
);
