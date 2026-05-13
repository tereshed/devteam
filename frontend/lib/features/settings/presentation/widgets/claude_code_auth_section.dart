import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/api/safe_error_message.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/settings/data/claude_code_auth_providers.dart';
import 'package:frontend/features/settings/domain/claude_code_auth_exceptions.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';

/// Sprint 15.m4: локализованный DateTime для UI (например, expires_at и last_refreshed_at).
String _fmtDateTime(BuildContext context, DateTime? dt) {
  if (dt == null) return '—';
  final localeTag = Localizations.localeOf(context).toLanguageTag();
  return DateFormat.yMMMd(localeTag).add_jm().format(dt.toLocal());
}

/// Sprint 15.31 — секция «Claude Code» в глобальных настройках.
///
/// Сценарии:
///  - нет подписки → показать кнопку «Войти по подписке» (запускает device-flow).
///  - есть подписка → показать статус (scopes, expires_at, last_refreshed_at) и кнопку «Отзыв».
///  - device-flow в процессе → показать user_code, verification_uri и поллить /callback.
class ClaudeCodeAuthSection extends ConsumerWidget {
  const ClaudeCodeAuthSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'claudeCodeAuthSection');
    final asyncStatus = ref.watch(claudeCodeAuthStatusProvider);
    final theme = Theme.of(context);

    return asyncStatus.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (err, _) => Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.claudeCodeAuthLoadError,
              style: theme.textTheme.titleMedium),
          const SizedBox(height: 8),
          SelectableText(safeErrorMessage(context, err)),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            onPressed: () => ref.invalidate(claudeCodeAuthStatusProvider),
            icon: const Icon(Icons.refresh),
            label: Text(l10n.retry),
          ),
        ],
      ),
      data: (status) => _ConnectedOrLoginView(status: status),
    );
  }
}

class _ConnectedOrLoginView extends ConsumerWidget {
  const _ConnectedOrLoginView({required this.status});

  final ClaudeCodeAuthStatus status;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'claudeCodeAuthSection');
    final theme = Theme.of(context);
    if (status.connected) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.claudeCodeAuthConnectedTitle,
              style: theme.textTheme.titleLarge),
          const SizedBox(height: 12),
          _StatusRow(
            label: l10n.claudeCodeAuthTokenType,
            value: status.tokenType,
          ),
          _StatusRow(
            label: l10n.claudeCodeAuthScopes,
            value: status.scopes,
          ),
          _StatusRow(
            label: l10n.claudeCodeAuthExpiresAt,
            value: _fmtDateTime(context, status.expiresAt),
          ),
          _StatusRow(
            label: l10n.claudeCodeAuthLastRefreshedAt,
            value: _fmtDateTime(context, status.lastRefreshedAt),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              FilledButton.tonalIcon(
                onPressed: () => _revoke(context, ref),
                icon: const Icon(Icons.logout),
                label: Text(l10n.claudeCodeAuthRevoke),
              ),
              const SizedBox(width: 12),
              OutlinedButton.icon(
                onPressed: () =>
                    ref.invalidate(claudeCodeAuthStatusProvider),
                icon: const Icon(Icons.refresh),
                label: Text(l10n.refresh),
              ),
            ],
          ),
        ],
      );
    }
    return _LoginFlow();
  }

  Future<void> _revoke(BuildContext context, WidgetRef ref) async {
    final l10n = requireAppLocalizations(context, where: 'claudeCodeAuthRevoke');
    // Sprint 15.F-M6: confirmation dialog (асимметрично с delete provider) — revoke
    // ломает аутентификацию всех проектов пользователя, нужен явный «yes».
    final confirm = await showDialog<bool>(
      context: context,
      builder: (_) => AlertDialog(
        title: Text(l10n.agentSandboxRevokeConfirmTitle),
        content: Text(l10n.agentSandboxRevokeConfirmBody),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton.tonal(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(l10n.claudeCodeAuthRevoke),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    if (!context.mounted) return;

    final repo = ref.read(claudeCodeAuthRepositoryProvider);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await repo.revoke();
      ref.invalidate(claudeCodeAuthStatusProvider);
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.claudeCodeAuthRevokeOK)),
      );
    } catch (err) {
      if (!context.mounted) return;
      messenger.showSnackBar(
        SnackBar(content: Text(safeErrorMessage(context, err))),
      );
    }
  }
}

class _StatusRow extends StatelessWidget {
  const _StatusRow({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 180, child: Text(label)),
          Expanded(child: SelectableText(value)),
        ],
      ),
    );
  }
}

/// Тонкий контейнер device-flow: храним active init + таймер поллинга.
class _LoginFlow extends ConsumerStatefulWidget {
  @override
  ConsumerState<_LoginFlow> createState() => _LoginFlowState();
}

class _LoginFlowState extends ConsumerState<_LoginFlow> {
  ClaudeCodeAuthInit? _init;
  bool _starting = false;
  Timer? _pollTimer;
  String? _statusMessage;
  bool _polling = false;

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  Future<void> _start() async {
    final repo = ref.read(claudeCodeAuthRepositoryProvider);
    setState(() {
      _starting = true;
      _statusMessage = null;
    });
    try {
      final init = await repo.initDeviceFlow();
      setState(() => _init = init);
      _schedulePoll(init);
    } catch (err) {
      if (!mounted) return;
      setState(() => _statusMessage = safeErrorMessage(context, err));
    } finally {
      if (mounted) setState(() => _starting = false);
    }
  }

  void _schedulePoll(ClaudeCodeAuthInit init) {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(
      Duration(seconds: init.intervalSeconds <= 0 ? 5 : init.intervalSeconds),
      (_) => _pollOnce(init),
    );
  }

  Future<void> _pollOnce(ClaudeCodeAuthInit init) async {
    if (_polling) return;
    _polling = true;
    try {
      final repo = ref.read(claudeCodeAuthRepositoryProvider);
      await repo.complete(init.deviceCode);
      _pollTimer?.cancel();
      ref.invalidate(claudeCodeAuthStatusProvider);
    } on ClaudeCodeAuthorizationPendingException {
      // pending — продолжаем поллинг (taймер тикает).
    } on ClaudeCodeAuthSlowDownException {
      // backend сказал, мы спрашиваем слишком часто; на следующем тике подождём ещё.
    } on ClaudeCodeAuthFlowEndedException catch (e) {
      _pollTimer?.cancel();
      if (mounted) {
        setState(() => _statusMessage = e.message);
      }
    } on ClaudeCodeAuthOwnerMismatchException catch (e) {
      _pollTimer?.cancel();
      if (mounted) {
        setState(() => _statusMessage = e.message);
      }
    } on ClaudeCodeAuthException catch (e) {
      // Sprint 15.B (F10): показываем sanitizedMessage из ApiException, не сырой $err.
      if (mounted) setState(() => _statusMessage = e.message);
    } finally {
      _polling = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'claudeCodeAuthLogin');
    final theme = Theme.of(context);
    if (_init == null) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(l10n.claudeCodeAuthDisconnectedTitle,
              style: theme.textTheme.titleLarge),
          const SizedBox(height: 12),
          Text(l10n.claudeCodeAuthDisconnectedHint),
          const SizedBox(height: 16),
          FilledButton.icon(
            onPressed: _starting ? null : _start,
            icon: const Icon(Icons.login),
            label: Text(_starting ? '…' : l10n.claudeCodeAuthLogin),
          ),
          if (_statusMessage != null) ...[
            const SizedBox(height: 12),
            SelectableText(_statusMessage!),
          ],
        ],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(l10n.claudeCodeAuthDeviceFlowTitle,
            style: theme.textTheme.titleLarge),
        const SizedBox(height: 12),
        Text(l10n.claudeCodeAuthEnterCodeHint),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: SelectableText(
                _init!.userCode,
                style: theme.textTheme.headlineSmall,
              ),
            ),
            IconButton(
              onPressed: () =>
                  Clipboard.setData(ClipboardData(text: _init!.userCode)),
              icon: const Icon(Icons.copy),
              tooltip: l10n.copy,
            ),
          ],
        ),
        const SizedBox(height: 12),
        // Sprint 15.B (F8): НЕ показываем verification_uri_complete на экране —
        // он содержит device_code, и скриншот/копипаст позволит злоумышленнику завершить
        // flow за пользователя (RFC 8628 §6.1). На экране — только базовый verification_uri,
        // user_code вводится вручную. _complete доступен через launchUrl (браузер откроет напрямую).
        Row(
          children: [
            Expanded(child: SelectableText(_init!.verificationURI)),
            IconButton(
              onPressed: () => launchUrl(Uri.parse(
                _init!.verificationURIComplete.isNotEmpty
                    ? _init!.verificationURIComplete
                    : _init!.verificationURI,
              )),
              icon: const Icon(Icons.open_in_new),
              tooltip: l10n.openInBrowser,
            ),
          ],
        ),
        const SizedBox(height: 12),
        Text(l10n.claudeCodeAuthWaiting,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurface.withValues(alpha: 0.72),
            )),
        if (_statusMessage != null) ...[
          const SizedBox(height: 12),
          SelectableText(_statusMessage!),
        ],
      ],
    );
  }
}
