import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_providers.dart';
import 'package:frontend/features/integrations/git/data/git_integrations_repository.dart';
import 'package:frontend/features/integrations/git/domain/git_integration_model.dart';
import 'package:url_launcher/url_launcher.dart';

/// Результат, возвращаемый успешным сабмитом BYO-диалога — содержит
/// authorize_url, который вызывающий код должен открыть в браузере.
class ConnectGitlabHostResult {
  const ConnectGitlabHostResult({required this.authorizeUrl});
  final String authorizeUrl;
}

/// Диалог подключения self-hosted GitLab (BYO Application).
///
/// Поля — host / client_id / client_secret + раскрывающийся блок инструкций
/// (oauth-setup-guide.md §5). Клиентская валидация — lightweight:
///   * host: непустой, https:// или http:// (последнее — для local dev), парсится в [Uri].
///   * client_id / client_secret: непустые.
///
/// Все «серьёзные» проверки (private IP, link-local, userinfo, DNS resolve)
/// делает бэкенд через `validateGitProviderHost` (3.1) — мы лишь красиво
/// отрисовываем ответ `400 invalid_host`.
///
/// `redirectUri` — какой URL вписать в инструкцию: совпадает с тем, что отправляем
/// в `POST /integrations/gitlab/auth/init` (бэк проверит match).
Future<ConnectGitlabHostResult?> showConnectGitlabHostDialog(
  BuildContext context,
  WidgetRef ref, {
  required String redirectUri,
}) async {
  return showDialog<ConnectGitlabHostResult>(
    context: context,
    builder: (ctx) =>
        _ConnectGitlabHostDialog(parentRef: ref, redirectUri: redirectUri),
  );
}

class _ConnectGitlabHostDialog extends ConsumerStatefulWidget {
  const _ConnectGitlabHostDialog({
    required this.parentRef,
    required this.redirectUri,
  });

  final WidgetRef parentRef;
  final String redirectUri;

  @override
  ConsumerState<_ConnectGitlabHostDialog> createState() =>
      _ConnectGitlabHostDialogState();
}

class _ConnectGitlabHostDialogState
    extends ConsumerState<_ConnectGitlabHostDialog> {
  final _formKey = GlobalKey<FormState>();
  final _hostController = TextEditingController();
  final _clientIdController = TextEditingController();
  final _clientSecretController = TextEditingController();
  bool _busy = false;
  bool _instructionsExpanded = false;
  String? _errorMessage;

  @override
  void dispose() {
    _hostController.dispose();
    _clientIdController.dispose();
    _clientSecretController.dispose();
    // Если диалог закрыли (Esc/клик мимо) во время активного init-запроса —
    // откатываем pending в disconnected. В success-пути `_busy=false`
    // выставляется до `Navigator.pop`, поэтому штатное закрытие сюда не падает.
    if (_busy) {
      widget.parentRef
          .read(gitIntegrationsControllerProvider.notifier)
          .rollbackToDisconnected(GitIntegrationProvider.gitlab);
    }
    super.dispose();
  }

  Future<void> _submit() async {
    if (_busy) {
      return;
    }
    if (_formKey.currentState?.validate() != true) {
      return;
    }
    setState(() {
      _busy = true;
      _errorMessage = null;
    });
    final controller = widget.parentRef.read(
      gitIntegrationsControllerProvider.notifier,
    );
    final host = _hostController.text.trim();
    try {
      // updateStateOnError=false: BYO-диалог сам отрисует серверную ошибку
      // в своём баннере, не трогая карточку GitLab в фоне.
      final authorizeUrl = await controller.initConnection(
        GitIntegrationProvider.gitlab,
        redirectUri: widget.redirectUri,
        host: host,
        byoClientId: _clientIdController.text.trim(),
        byoClientSecret: _clientSecretController.text.trim(),
        updateStateOnError: false,
      );
      if (!mounted) {
        return;
      }
      // Гасим `_busy` ДО pop — иначе dispose откатит pending в disconnected
      // и стёр бы только что выставленный init'ом pending.
      _busy = false;
      Navigator.of(
        context,
      ).pop(ConnectGitlabHostResult(authorizeUrl: authorizeUrl));
    } on GitIntegrationsException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _errorMessage = _localizedServerError(e);
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _errorMessage = e.toString();
      });
    }
  }

  String _localizedServerError(GitIntegrationsException e) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectGitlabHostDialog._serverError',
    );
    switch (e.errorCode) {
      case 'invalid_host':
        return l10n.integrationsGitReasonInvalidHost;
      case 'oauth_not_configured':
        return l10n.integrationsGitReasonOauthNotConfigured;
      case 'provider_unreachable':
        return l10n.integrationsGitReasonProviderUnreachable;
      default:
        return l10n.integrationsGitReasonUnknown(e.message);
    }
  }

  String? _validateHost(String? raw) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectGitlabHostDialog._validateHost',
    );
    final value = (raw ?? '').trim();
    if (value.isEmpty) {
      return l10n.integrationsGitlabHostValidationHostRequired;
    }
    final parsed = Uri.tryParse(value);
    if (parsed == null || parsed.host.isEmpty) {
      return l10n.integrationsGitlabHostValidationHostFormat;
    }
    if (parsed.scheme != 'https' && parsed.scheme != 'http') {
      return l10n.integrationsGitlabHostValidationHostScheme;
    }
    return null;
  }

  String? _validateRequired(String? raw, String message) {
    if ((raw ?? '').trim().isEmpty) {
      return message;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectGitlabHostDialog',
    );
    final theme = Theme.of(context);
    return AlertDialog(
      title: Text(l10n.integrationsGitlabHostDialogTitle),
      content: Form(
        key: _formKey,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minWidth: 360, maxWidth: 520),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextFormField(
                  controller: _hostController,
                  enabled: !_busy,
                  keyboardType: TextInputType.url,
                  autocorrect: false,
                  decoration: InputDecoration(
                    labelText: l10n.integrationsGitlabHostFieldHost,
                    helperText: l10n.integrationsGitlabHostFieldHostHint,
                  ),
                  validator: _validateHost,
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _clientIdController,
                  enabled: !_busy,
                  autocorrect: false,
                  decoration: InputDecoration(
                    labelText: l10n.integrationsGitlabHostFieldClientId,
                  ),
                  validator: (v) => _validateRequired(
                    v,
                    l10n.integrationsGitlabHostValidationClientIdRequired,
                  ),
                ),
                const SizedBox(height: 12),
                TextFormField(
                  controller: _clientSecretController,
                  enabled: !_busy,
                  obscureText: true,
                  autocorrect: false,
                  decoration: InputDecoration(
                    labelText: l10n.integrationsGitlabHostFieldClientSecret,
                    helperText: l10n.integrationsGitlabHostFieldSecretHint,
                  ),
                  validator: (v) => _validateRequired(
                    v,
                    l10n.integrationsGitlabHostValidationClientSecretRequired,
                  ),
                ),
                const SizedBox(height: 12),
                _Instructions(
                  redirectUri: widget.redirectUri,
                  expanded: _instructionsExpanded,
                  onToggle: () => setState(
                    () => _instructionsExpanded = !_instructionsExpanded,
                  ),
                ),
                if (_errorMessage != null) ...[
                  const SizedBox(height: 12),
                  Container(
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.errorContainer,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(
                          Icons.error_outline,
                          color: theme.colorScheme.error,
                        ),
                        const SizedBox(width: 8),
                        Expanded(
                          child: Text(
                            _errorMessage!,
                            style: TextStyle(
                              color: theme.colorScheme.onErrorContainer,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.integrationsGitlabHostCancelCta),
        ),
        FilledButton(
          onPressed: _busy ? null : _submit,
          child: _busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.integrationsGitlabHostSubmitCta),
        ),
      ],
    );
  }
}

class _Instructions extends StatelessWidget {
  const _Instructions({
    required this.redirectUri,
    required this.expanded,
    required this.onToggle,
  });

  final String redirectUri;
  final bool expanded;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_Instructions');
    final theme = Theme.of(context);
    final steps = <String>[
      l10n.integrationsGitlabHostInstructionsStep1,
      l10n.integrationsGitlabHostInstructionsStep2,
      l10n.integrationsGitlabHostInstructionsStep3(redirectUri),
      l10n.integrationsGitlabHostInstructionsStep4,
      l10n.integrationsGitlabHostInstructionsStep5,
    ];
    return Card(
      margin: EdgeInsets.zero,
      color: theme.colorScheme.surfaceContainerHighest,
      elevation: 0,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          InkWell(
            onTap: onToggle,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
              child: Row(
                children: [
                  Icon(
                    expanded ? Icons.expand_less : Icons.expand_more,
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      l10n.integrationsGitlabHostInstructionsToggle,
                      style: theme.textTheme.titleSmall,
                    ),
                  ),
                ],
              ),
            ),
          ),
          if (expanded)
            Padding(
              padding: const EdgeInsets.fromLTRB(36, 0, 12, 12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (var i = 0; i < steps.length; i++)
                    Padding(
                      padding: const EdgeInsets.symmetric(vertical: 2),
                      child: Text(
                        '${i + 1}. ${steps[i]}',
                        style: theme.textTheme.bodySmall,
                      ),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

/// Удобный хелпер, вызываемый screen'ом: показать диалог и, если получен
/// authorize_url, открыть его в браузере. Возвращает true, если flow стартовал.
///
/// На любой провал открытия URL (невалидный URL или `launchUrl()==false`)
/// откатывает локальный `pending → disconnected`, иначе кнопка «Connect»
/// в карточке GitLab осталась бы навсегда залочена в busy.
Future<bool> showAndLaunchConnectGitlabHost(
  BuildContext context,
  WidgetRef ref, {
  required String redirectUri,
}) async {
  final result = await showConnectGitlabHostDialog(
    context,
    ref,
    redirectUri: redirectUri,
  );
  if (result == null) {
    return false;
  }
  final controller = ref.read(gitIntegrationsControllerProvider.notifier);
  final uri = Uri.tryParse(result.authorizeUrl);
  if (uri == null) {
    controller.rollbackToDisconnected(GitIntegrationProvider.gitlab);
    return false;
  }
  final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
  if (!ok) {
    controller.rollbackToDisconnected(GitIntegrationProvider.gitlab);
    if (context.mounted) {
      final l10n = requireAppLocalizations(
        context,
        where: 'showAndLaunchConnectGitlabHost',
      );
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(l10n.integrationsGitBrowserOpenFailed(uri.toString())),
        ),
      );
    }
    return false;
  }
  return true;
}
