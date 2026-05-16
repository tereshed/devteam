import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:url_launcher/url_launcher.dart';

/// Диалог OAuth device-flow для Claude Code (UI Refactoring §5 Этап 2 / §4a.5).
///
/// Поведение:
///   1. На open вызывает `POST /claude-code/auth/init` и получает `user_code` + URL.
///   2. Кнопка «Open browser» → `url_launcher` уводит юзера на Anthropic.
///   3. Дальше ждём `IntegrationConnectionChanged` через WS — НЕ поллинг.
///   4. Состояние pending ограничено 20 минутами — после таймаута переход в `disconnected`.
///   5. Любой error/cancel из WS → понятный UI с кнопкой «Try again».
Future<void> showConnectClaudeCodeDialog(
  BuildContext context,
  WidgetRef ref,
) async {
  await showDialog<void>(
    context: context,
    barrierDismissible: true,
    builder: (ctx) => _ConnectClaudeCodeDialog(parentRef: ref),
  );
}

enum _OAuthDialogPhase { initializing, awaiting, success, error, timeout }

class _ConnectClaudeCodeDialog extends ConsumerStatefulWidget {
  const _ConnectClaudeCodeDialog({required this.parentRef});

  final WidgetRef parentRef;

  @override
  ConsumerState<_ConnectClaudeCodeDialog> createState() =>
      _ConnectClaudeCodeDialogState();
}

class _ConnectClaudeCodeDialogState
    extends ConsumerState<_ConnectClaudeCodeDialog> {
  /// §4a.5: pending не должен висеть дольше 20 минут.
  static const _pendingTimeout = Duration(minutes: 20);

  _OAuthDialogPhase _phase = _OAuthDialogPhase.initializing;
  ClaudeCodeOAuthInit? _init;
  String? _errorMessage;
  String? _errorReason;
  StreamSubscription<WsClientEvent>? _wsSub;
  Timer? _timeoutTimer;

  @override
  void initState() {
    super.initState();
    _startOAuth();
  }

  @override
  void dispose() {
    _timeoutTimer?.cancel();
    unawaited(_wsSub?.cancel());
    super.dispose();
  }

  Future<void> _startOAuth() async {
    setState(() {
      _phase = _OAuthDialogPhase.initializing;
      _errorMessage = null;
      _errorReason = null;
    });
    final repo = widget.parentRef.read(llmIntegrationsRepositoryProvider);
    final controller = widget.parentRef.read(llmIntegrationsControllerProvider);
    try {
      final init = await repo.initClaudeCodeOAuth();
      controller.applyLocal(
        const LlmProviderConnection(
          provider: LlmIntegrationProvider.claudeCodeOAuth,
          status: LlmProviderConnectionStatus.pending,
        ),
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _init = init;
        _phase = _OAuthDialogPhase.awaiting;
      });
      _subscribeToWs();
      _scheduleTimeout();
    } on LlmIntegrationsException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _phase = _OAuthDialogPhase.error;
        _errorReason = e.errorCode;
        _errorMessage = e.message;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _phase = _OAuthDialogPhase.error;
        _errorMessage = e.toString();
      });
    }
  }

  void _subscribeToWs() {
    unawaited(_wsSub?.cancel());
    final ws = widget.parentRef.read(webSocketServiceProvider);
    _wsSub = ws.events.listen((ev) {
      if (ev is! WsClientEventServer) {
        return;
      }
      final inner = ev.event;
      if (inner is! WsServerEventIntegrationStatus) {
        return;
      }
      final integration = inner.value;
      if (integration.provider != 'claude_code_oauth') {
        return;
      }
      switch (integration.status) {
        case WsIntegrationStatus.connected:
          _onSuccess();
          break;
        case WsIntegrationStatus.error:
          _onWsError(integration.reason);
          break;
        case WsIntegrationStatus.disconnected:
        case WsIntegrationStatus.pending:
          // Промежуточные — оставляем UI в awaiting.
          break;
      }
    });
  }

  void _scheduleTimeout() {
    _timeoutTimer?.cancel();
    _timeoutTimer = Timer(_pendingTimeout, () {
      if (!mounted) {
        return;
      }
      final controller = widget.parentRef.read(
        llmIntegrationsControllerProvider,
      );
      controller.applyLocal(
        const LlmProviderConnection(
          provider: LlmIntegrationProvider.claudeCodeOAuth,
          status: LlmProviderConnectionStatus.disconnected,
        ),
      );
      setState(() {
        _phase = _OAuthDialogPhase.timeout;
      });
    });
  }

  void _onSuccess() {
      if (!mounted) {
        return;
      }
    _timeoutTimer?.cancel();
    final controller = widget.parentRef.read(llmIntegrationsControllerProvider);
    controller.applyLocal(
      const LlmProviderConnection(
        provider: LlmIntegrationProvider.claudeCodeOAuth,
        status: LlmProviderConnectionStatus.connected,
      ),
    );
    setState(() {
      _phase = _OAuthDialogPhase.success;
    });
    // Закрываем диалог через короткий тик, чтобы юзер увидел финальное состояние.
    Future<void>.delayed(const Duration(milliseconds: 500), () {
      if (mounted) {
        Navigator.of(context).pop();
      }
    });
  }

  void _onWsError(String? reason) {
      if (!mounted) {
        return;
      }
    _timeoutTimer?.cancel();
    final controller = widget.parentRef.read(llmIntegrationsControllerProvider);
    controller.applyLocal(
      LlmProviderConnection(
        provider: LlmIntegrationProvider.claudeCodeOAuth,
        status: LlmProviderConnectionStatus.error,
        reason: reason,
      ),
    );
    setState(() {
      _phase = _OAuthDialogPhase.error;
      _errorReason = reason;
      _errorMessage = null;
    });
  }

  Future<void> _openBrowser() async {
    final init = _init;
    if (init == null) {
      return;
    }
    final uri = Uri.parse(
      init.verificationUriComplete.isNotEmpty
          ? init.verificationUriComplete
          : init.verificationUri,
    );
    final ok = await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!ok && mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(uri.toString())));
    }
  }

  Future<void> _copyCode() async {
    final code = _init?.userCode;
    if (code == null || code.isEmpty) {
      return;
    }
    await Clipboard.setData(ClipboardData(text: code));
  }

  String _reasonText(BuildContext context, String? reason) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectClaudeCodeDialog._reason',
    );
    switch (reason) {
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
        return l10n.integrationsLlmReasonUnknown(reason ?? '');
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectClaudeCodeDialog',
    );
    return AlertDialog(
      title: Text(l10n.integrationsLlmClaudeCodeOAuthTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(minWidth: 360, maxWidth: 520),
        child: _content(context, l10n),
      ),
      actions: _actions(context, l10n),
    );
  }

  Widget _content(BuildContext context, dynamic l10n) {
    final theme = Theme.of(context);
    switch (_phase) {
      case _OAuthDialogPhase.initializing:
        return const Center(
          child: Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: CircularProgressIndicator(),
          ),
        );
      case _OAuthDialogPhase.awaiting:
        final init = _init!;
        return Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(l10n.integrationsLlmClaudeCodeOAuthStep1),
            const SizedBox(height: 16),
            Row(
              children: [
                Text(
                  l10n.integrationsLlmClaudeCodeOAuthCode,
                  style: theme.textTheme.titleSmall,
                ),
                const SizedBox(width: 8),
                SelectableText(
                  init.userCode,
                  style: theme.textTheme.titleMedium?.copyWith(
                    fontFamily: 'monospace',
                  ),
                ),
                IconButton(
                  tooltip: l10n.integrationsLlmClaudeCodeOAuthCopy,
                  icon: const Icon(Icons.copy, size: 18),
                  onPressed: _copyCode,
                ),
              ],
            ),
            const SizedBox(height: 16),
            Row(
              children: [
                const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    l10n.integrationsLlmClaudeCodeOAuthWaiting,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
              ],
            ),
          ],
        );
      case _OAuthDialogPhase.success:
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: 16),
          child: Row(
            children: [
              Icon(Icons.check_circle, color: theme.colorScheme.primary),
              const SizedBox(width: 12),
              Text(l10n.integrationStatusConnected),
            ],
          ),
        );
      case _OAuthDialogPhase.error:
        return Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.error_outline, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(
              _errorMessage != null && _errorReason == null
                  ? _errorMessage!
                  : _reasonText(context, _errorReason),
            ),
          ],
        );
      case _OAuthDialogPhase.timeout:
        return Padding(
          padding: const EdgeInsets.symmetric(vertical: 16),
          child: Text(l10n.integrationsLlmClaudeCodeOAuthTimeout),
        );
    }
  }

  List<Widget> _actions(BuildContext context, dynamic l10n) {
    switch (_phase) {
      case _OAuthDialogPhase.initializing:
        return [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.integrationsLlmDialogCancel),
          ),
        ];
      case _OAuthDialogPhase.awaiting:
        return [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.integrationsLlmDialogCancel),
          ),
          FilledButton.icon(
            onPressed: _openBrowser,
            icon: const Icon(Icons.open_in_new, size: 18),
            label: Text(l10n.integrationsLlmClaudeCodeOpenBrowser),
          ),
        ];
      case _OAuthDialogPhase.success:
        return [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.integrationsLlmDialogSave),
          ),
        ];
      case _OAuthDialogPhase.error:
      case _OAuthDialogPhase.timeout:
        return [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
            child: Text(l10n.integrationsLlmDialogCancel),
          ),
          FilledButton(
            onPressed: _startOAuth,
            child: Text(l10n.integrationsLlmRetry),
          ),
        ];
    }
  }
}
