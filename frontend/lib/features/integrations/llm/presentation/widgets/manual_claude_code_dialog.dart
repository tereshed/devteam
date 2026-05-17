import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

/// Диалог для ручного ввода готового OAuth-токена Claude Code (BYO setup-token).
///
/// Сохраняет токен через `PUT /claude-code/auth/manual-token` и обновляет
/// локальный стейт LLM-провайдеров. Используется, когда device-flow на бэке не
/// настроен (`CLAUDE_CODE_OAUTH_CLIENT_ID` пуст) или у юзера уже есть токен.
Future<void> showManualClaudeCodeDialog(
  BuildContext context,
  WidgetRef ref,
) async {
  await showDialog<void>(
    context: context,
    barrierDismissible: true,
    builder: (ctx) => _ManualClaudeCodeDialog(parentRef: ref),
  );
}

class _ManualClaudeCodeDialog extends ConsumerStatefulWidget {
  const _ManualClaudeCodeDialog({required this.parentRef});

  final WidgetRef parentRef;

  @override
  ConsumerState<_ManualClaudeCodeDialog> createState() =>
      _ManualClaudeCodeDialogState();
}

class _ManualClaudeCodeDialogState
    extends ConsumerState<_ManualClaudeCodeDialog> {
  final _formKey = GlobalKey<FormState>();
  final _accessCtrl = TextEditingController();
  final _refreshCtrl = TextEditingController();
  bool _busy = false;
  String? _errorText;

  @override
  void dispose() {
    _accessCtrl.dispose();
    _refreshCtrl.dispose();
    super.dispose();
  }

  Future<void> _onSave() async {
    if (_busy) {
      return;
    }
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    setState(() {
      _busy = true;
      _errorText = null;
    });
    final repo = widget.parentRef.read(llmIntegrationsRepositoryProvider);
    final controller = widget.parentRef.read(
      llmIntegrationsControllerProvider,
    );
    try {
      final status = await repo.saveClaudeCodeManualToken(
        accessToken: _accessCtrl.text.trim(),
        refreshToken: _refreshCtrl.text.trim().isEmpty
            ? null
            : _refreshCtrl.text.trim(),
      );
      controller.applyLocal(
        LlmProviderConnection(
          provider: LlmIntegrationProvider.claudeCodeOAuth,
          status: status.connected
              ? LlmProviderConnectionStatus.connected
              : LlmProviderConnectionStatus.error,
        ),
      );
      if (!mounted) {
        return;
      }
      Navigator.of(context).pop();
    } on LlmIntegrationsException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _errorText = e.message;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _errorText = e.toString();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ManualClaudeCodeDialog',
    );
    final theme = Theme.of(context);
    return AlertDialog(
      title: Text(l10n.integrationsLlmClaudeCodeManualTitle),
      content: ConstrainedBox(
        constraints: const BoxConstraints(minWidth: 360, maxWidth: 520),
        child: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                l10n.integrationsLlmClaudeCodeManualHint,
                style: theme.textTheme.bodySmall?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                ),
              ),
              const SizedBox(height: 16),
              TextFormField(
                controller: _accessCtrl,
                obscureText: true,
                enableSuggestions: false,
                autocorrect: false,
                minLines: 1,
                maxLines: 1,
                decoration: InputDecoration(
                  labelText:
                      l10n.integrationsLlmClaudeCodeManualAccessField,
                ),
                validator: (v) => (v == null || v.trim().isEmpty)
                    ? l10n.integrationsLlmClaudeCodeManualAccessRequired
                    : null,
              ),
              const SizedBox(height: 12),
              TextFormField(
                controller: _refreshCtrl,
                obscureText: true,
                enableSuggestions: false,
                autocorrect: false,
                decoration: InputDecoration(
                  labelText:
                      l10n.integrationsLlmClaudeCodeManualRefreshField,
                ),
              ),
              if (_errorText != null) ...[
                const SizedBox(height: 12),
                Text(
                  _errorText!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.error,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(),
          child: Text(l10n.integrationsLlmDialogCancel),
        ),
        FilledButton(
          onPressed: _busy ? null : _onSave,
          child: _busy
              ? const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.integrationsLlmDialogSave),
        ),
      ],
    );
  }
}
