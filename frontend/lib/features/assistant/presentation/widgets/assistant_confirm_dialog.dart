import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/l10n/require.dart';

/// Inline-карточка подтверждения destructive операции (Sprint 21 §12 frontend).
///
/// Вставляется в конец списка сообщений, когда приходит
/// [WsAssistantConfirmRequestEvent]. Не модальный диалог — пользователь
/// видит её в потоке чата.
class AssistantConfirmDialog extends StatelessWidget {
  const AssistantConfirmDialog({
    super.key,
    required this.event,
    required this.onApprove,
    required this.onDeny,
    this.busy = false,
  });

  final WsAssistantConfirmRequestEvent event;
  final VoidCallback onApprove;
  final VoidCallback onDeny;

  /// Идёт `POST /confirm` — дизейблим кнопки, чтобы дабл-клик не дёргал
  /// бэкенд (он сам идемпотентен, но UX-нагрузку лучше снизить).
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AssistantConfirmDialog',
    );
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final summary = (event.summary != null && event.summary!.isNotEmpty)
        ? event.summary!
        : l10n.assistantConfirmSummaryFallback(event.toolName);
    final args = event.arguments.isEmpty
        ? null
        : const JsonEncoder.withIndent('  ').convert(event.arguments);

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      color: scheme.tertiaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.warning_amber_rounded,
                    color: scheme.onTertiaryContainer),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.assistantConfirmTitle,
                    style: theme.textTheme.titleSmall?.copyWith(
                      color: scheme.onTertiaryContainer,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              summary,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: scheme.onTertiaryContainer,
              ),
            ),
            if (args != null) ...[
              const SizedBox(height: 8),
              Container(
                width: double.infinity,
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(6),
                ),
                child: SelectableText(
                  args,
                  style: theme.textTheme.bodySmall
                      ?.copyWith(fontFamily: 'monospace'),
                ),
              ),
            ],
            const SizedBox(height: 12),
            Row(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                TextButton(
                  onPressed: busy ? null : onDeny,
                  child: Text(l10n.assistantConfirmDeny),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  onPressed: busy ? null : onApprove,
                  child: Text(l10n.assistantConfirmApprove),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
