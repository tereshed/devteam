import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/settings/data/claude_code_auth_providers.dart';
import 'package:frontend/features/settings/data/claude_code_auth_repository.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';
import 'package:url_launcher/url_launcher.dart';

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
          SelectableText('$err'),
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
            value: status.expiresAt?.toIso8601String() ?? '—',
          ),
          _StatusRow(
            label: l10n.claudeCodeAuthLastRefreshedAt,
            value: status.lastRefreshedAt?.toIso8601String() ?? '—',
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
    final repo = ref.read(claudeCodeAuthRepositoryProvider);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await repo.revoke();
      ref.invalidate(claudeCodeAuthStatusProvider);
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.claudeCodeAuthRevokeOK)),
      );
    } catch (err) {
      messenger.showSnackBar(SnackBar(content: Text('$err')));
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
      setState(() => _statusMessage = '$err');
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
    } on DioException catch (e) {
      final code = e.response?.statusCode;
      if (code == 202) {
        // authorization_pending — продолжаем поллинг.
      } else if (code == 410 || code == 400) {
        _pollTimer?.cancel();
        if (mounted) {
          setState(() => _statusMessage = e.response?.statusMessage ?? 'expired');
        }
      } else {
        if (mounted) setState(() => _statusMessage = '$e');
      }
    } catch (err) {
      if (mounted) setState(() => _statusMessage = '$err');
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
        Row(
          children: [
            Expanded(
              child: SelectableText(_init!.verificationURIComplete.isNotEmpty
                  ? _init!.verificationURIComplete
                  : _init!.verificationURI),
            ),
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
