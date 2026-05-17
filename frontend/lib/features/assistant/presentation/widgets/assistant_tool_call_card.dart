import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Tool-call карточка (Sprint 21 §8 frontend).
///
/// Свёрнута по умолчанию: `🔧 tool_name`. Раскрытие показывает arguments JSON
/// и (если есть) result JSON со статус-бейджем.
class AssistantToolCallCard extends StatefulWidget {
  const AssistantToolCallCard({
    super.key,
    required this.assistantMessage,
    this.toolResult,
  });

  /// Assistant-row с tool_call_id (см. бэкенд OnAssistantMessage).
  final AssistantMessageModel assistantMessage;

  /// tool-row с тем же tool_call_id (см. OnToolResult). Может быть null —
  /// например, ждём confirm или WS-эвент ещё не пришёл.
  final AssistantMessageModel? toolResult;

  @override
  State<AssistantToolCallCard> createState() => _AssistantToolCallCardState();
}

class _AssistantToolCallCardState extends State<AssistantToolCallCard> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AssistantToolCallCard',
    );
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final toolName = widget.assistantMessage.toolName ?? '?';
    final result = widget.toolResult;
    final status = _extractStatus(result);

    return Card(
      margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      color: scheme.surfaceContainerLow,
      child: InkWell(
        onTap: () => setState(() => _expanded = !_expanded),
        child: AnimatedSize(
          duration: const Duration(milliseconds: 180),
          alignment: Alignment.topCenter,
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Icon(Icons.build_circle_outlined, size: 18),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        l10n.assistantToolCallTitle(toolName),
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    if (status != null) ...[
                      const SizedBox(width: 8),
                      _StatusBadge(status: status, label: _statusLabel(l10n, status)),
                    ],
                    const SizedBox(width: 4),
                    Icon(
                      _expanded ? Icons.expand_less : Icons.expand_more,
                      size: 18,
                    ),
                  ],
                ),
                if (_expanded) ...[
                  const SizedBox(height: 8),
                  _JsonBlock(
                    label: l10n.assistantToolArgumentsLabel,
                    json: widget.assistantMessage.toolArguments,
                  ),
                  if (result != null && result.toolResult != null) ...[
                    const SizedBox(height: 8),
                    _JsonBlock(
                      label: l10n.assistantToolResultLabel,
                      json: result.toolResult,
                    ),
                  ],
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  String? _extractStatus(AssistantMessageModel? result) {
    if (result == null) return null;
    final r = result.toolResult;
    if (r == null) return 'pending';
    final raw = r['status'];
    if (raw is String && raw.isNotEmpty) return raw;
    return null;
  }

  String _statusLabel(AppLocalizations l10n, String status) {
    switch (status) {
      case 'ok':
        return l10n.assistantToolResultStatusOk;
      case 'forbidden':
        return l10n.assistantToolResultStatusForbidden;
      case 'denied':
        return l10n.assistantToolResultStatusDenied;
      case 'truncated':
        return l10n.assistantToolResultStatusTruncated;
      case 'pending':
        return l10n.assistantToolResultStatusPending;
      case 'error':
        return l10n.assistantToolResultStatusError;
      default:
        return status;
    }
  }
}

class _StatusBadge extends StatelessWidget {
  const _StatusBadge({required this.status, required this.label});

  final String status;
  final String label;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final (bg, fg) = switch (status) {
      'ok' => (scheme.tertiaryContainer, scheme.onTertiaryContainer),
      'pending' => (scheme.secondaryContainer, scheme.onSecondaryContainer),
      'forbidden' || 'denied' || 'error' => (scheme.errorContainer, scheme.onErrorContainer),
      _ => (scheme.surfaceContainerHighest, scheme.onSurfaceVariant),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelSmall?.copyWith(color: fg),
      ),
    );
  }
}

class _JsonBlock extends StatelessWidget {
  const _JsonBlock({required this.label, required this.json});

  final String label;
  final Map<String, dynamic>? json;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final pretty = (json == null || json!.isEmpty)
        ? '—'
        : const JsonEncoder.withIndent('  ').convert(json);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: theme.textTheme.labelSmall?.copyWith(
            color: scheme.onSurfaceVariant,
            fontWeight: FontWeight.w600,
          ),
        ),
        const SizedBox(height: 2),
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(6),
          ),
          child: SelectableText(
            pretty,
            style: theme.textTheme.bodySmall?.copyWith(
              fontFamily: 'monospace',
            ),
          ),
        ),
      ],
    );
  }
}
