import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_repository.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';

/// Диалог ввода API-ключа для API-key LLM-провайдера (Anthropic/OpenAI/...).
///
/// При успешном сохранении делает локальное обновление состояния (controller.applyLocal)
/// — UI обновится сразу, без ожидания WS-эхо.
Future<void> showConnectApiKeyDialog(
  BuildContext context,
  WidgetRef ref, {
  required LlmIntegrationProvider provider,
}) async {
  await showDialog<void>(
    context: context,
    builder: (ctx) => _ConnectApiKeyDialog(provider: provider, parentRef: ref),
  );
}

class _ConnectApiKeyDialog extends ConsumerStatefulWidget {
  const _ConnectApiKeyDialog({required this.provider, required this.parentRef});

  final LlmIntegrationProvider provider;
  final WidgetRef parentRef;

  @override
  ConsumerState<_ConnectApiKeyDialog> createState() =>
      _ConnectApiKeyDialogState();
}

class _ConnectApiKeyDialogState extends ConsumerState<_ConnectApiKeyDialog> {
  final _formKey = GlobalKey<FormState>();
  final _controller = TextEditingController();
  bool _busy = false;
  String? _errorMessage;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _save() async {
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
    final repo = widget.parentRef.read(llmIntegrationsRepositoryProvider);
    final controller = widget.parentRef.read(llmIntegrationsControllerProvider);
    try {
      await repo.setApiKey(
        provider: widget.provider,
        apiKey: _controller.text.trim(),
      );
      controller.applyLocal(
        LlmProviderConnection(
          provider: widget.provider,
          status: LlmProviderConnectionStatus.connected,
        ),
      );
      // Подтягиваем актуальную masked_preview одним фетчем.
      unawaited(controller.refresh());
      if (mounted) {
        Navigator.of(context).pop();
      }
    } on LlmIntegrationsException catch (e) {
      setState(() {
        _busy = false;
        _errorMessage = e.message;
      });
    } catch (e) {
      setState(() {
        _busy = false;
        _errorMessage = e.toString();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: '_ConnectApiKeyDialog',
    );
    final providerLabel = _providerLabel(l10n, widget.provider);

    return AlertDialog(
      title: Text(l10n.integrationsLlmDialogApiKeyTitle(providerLabel)),
      content: Form(
        key: _formKey,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minWidth: 320, maxWidth: 480),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextFormField(
                controller: _controller,
                obscureText: true,
                enabled: !_busy,
                decoration: InputDecoration(
                  labelText: l10n.integrationsLlmDialogApiKeyField,
                  helperText: l10n.integrationsLlmDialogApiKeyHint,
                ),
                validator: (raw) {
                  if ((raw ?? '').trim().isEmpty) {
                    return l10n.integrationsLlmDialogApiKeyRequired;
                  }
                  return null;
                },
                onFieldSubmitted: (_) => _save(),
              ),
              if (_errorMessage != null) ...[
                const SizedBox(height: 12),
                Text(
                  _errorMessage!,
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
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
          onPressed: _busy ? null : _save,
          child: _busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(l10n.integrationsLlmDialogSave),
        ),
      ],
    );
  }

  String _providerLabel(dynamic l10n, LlmIntegrationProvider p) {
    switch (p) {
      case LlmIntegrationProvider.anthropic:
        return l10n.llmProviderAnthropic;
      case LlmIntegrationProvider.openai:
        return l10n.llmProviderOpenAi;
      case LlmIntegrationProvider.openrouter:
        return l10n.llmProviderOpenRouter;
      case LlmIntegrationProvider.deepseek:
        return l10n.llmProviderDeepSeek;
      case LlmIntegrationProvider.zhipu:
        return l10n.llmProviderZhipu;
      case LlmIntegrationProvider.gemini:
        return 'Gemini';
      case LlmIntegrationProvider.qwen:
        return 'Qwen';
      case LlmIntegrationProvider.claudeCodeOAuth:
        return l10n.llmProviderClaudeCode;
    }
  }
}
