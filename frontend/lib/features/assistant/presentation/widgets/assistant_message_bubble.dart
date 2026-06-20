import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Сообщение ассистента/пользователя/системы (Sprint 21 §8, redesign).
///
/// Современная раскладка (стиль ChatGPT/Claude):
/// - пользователь — компактный bubble справа;
/// - ассистент — на всю ширину с аватаром (markdown/таблицам есть место);
/// - система — приглушённая строка с иконкой.
/// Ярлыки ролей убраны: роль читается по выравниванию/аватару.
///
/// Текст выделяется мышью (MarkdownBody.selectable). Дополнительно у каждого
/// сообщения есть кнопка «Копировать» — гарантированный способ забрать текст
/// независимо от платформы/жестов.
class AssistantMessageBubble extends StatelessWidget {
  const AssistantMessageBubble({super.key, required this.message});

  final AssistantMessageModel message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final isUser = message.isUser;
    final isSystem = message.isSystem;

    final chatRole = switch (message.role) {
      assistantMessageRoleUser => 'user',
      assistantMessageRoleAssistant => 'assistant',
      assistantMessageRoleSystem => 'system',
      _ => 'assistant',
    };

    if (isUser) {
      return Padding(
        padding: const EdgeInsets.fromLTRB(40, 4, 12, 4),
        child: Align(
          alignment: Alignment.centerRight,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            mainAxisSize: MainAxisSize.min,
            children: [
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 300),
                child: Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                  decoration: BoxDecoration(
                    color: scheme.primary,
                    borderRadius: const BorderRadius.only(
                      topLeft: Radius.circular(16),
                      topRight: Radius.circular(16),
                      bottomLeft: Radius.circular(16),
                      bottomRight: Radius.circular(4),
                    ),
                  ),
                  child: _content(theme, scheme.onPrimary, chatRole, isSystem),
                ),
              ),
              _copyButton(context, message.content ?? ''),
            ],
          ),
        ),
      );
    }

    // assistant / system — на всю ширину с аватаром.
    final fg = isSystem ? scheme.onSurfaceVariant : scheme.onSurface;
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 10),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          CircleAvatar(
            radius: 13,
            backgroundColor: isSystem
                ? scheme.surfaceContainerHighest
                : scheme.primary.withValues(alpha: 0.15),
            child: Icon(
              isSystem ? Icons.info_outline : Icons.auto_awesome,
              size: 15,
              color: isSystem ? scheme.onSurfaceVariant : scheme.primary,
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _content(theme, fg, chatRole, isSystem),
                _copyButton(context, message.content ?? ''),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _content(ThemeData theme, Color fg, String chatRole, bool isSystem) {
    return Theme(
      data: theme.copyWith(
        textTheme: theme.textTheme.copyWith(
          bodyMedium: theme.textTheme.bodyMedium?.copyWith(
            color: fg,
            fontStyle: isSystem ? FontStyle.italic : FontStyle.normal,
          ),
          titleLarge: theme.textTheme.titleLarge?.copyWith(color: fg),
          titleMedium: theme.textTheme.titleMedium?.copyWith(color: fg),
          titleSmall: theme.textTheme.titleSmall?.copyWith(color: fg),
          bodyLarge: theme.textTheme.bodyLarge?.copyWith(color: fg),
          bodySmall: theme.textTheme.bodySmall?.copyWith(color: fg),
        ),
        colorScheme: theme.colorScheme.copyWith(
          primary: fg,
          onSurface: fg,
          onSurfaceVariant: fg.withValues(alpha: 0.8),
        ),
      ),
      child: ChatMessage(
        role: chatRole,
        content: message.content ?? '',
        messageId: message.id,
      ),
    );
  }

  /// Кнопка-копирование всего текста сообщения в буфер обмена.
  Widget _copyButton(BuildContext context, String text) {
    if (text.trim().isEmpty) {
      return const SizedBox.shrink();
    }
    final l10n = AppLocalizations.of(context)!;
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.only(top: 2),
      child: IconButton(
        icon: const Icon(Icons.copy_rounded, size: 15),
        visualDensity: VisualDensity.compact,
        padding: EdgeInsets.zero,
        constraints: const BoxConstraints(minWidth: 30, minHeight: 26),
        color: scheme.onSurfaceVariant,
        tooltip: l10n.assistantCopyMessage,
        onPressed: () {
          Clipboard.setData(ClipboardData(text: text));
          ScaffoldMessenger.maybeOf(context)?.showSnackBar(
            SnackBar(
              content: Text(l10n.assistantCopied),
              duration: const Duration(seconds: 1),
            ),
          );
        },
      ),
    );
  }
}
