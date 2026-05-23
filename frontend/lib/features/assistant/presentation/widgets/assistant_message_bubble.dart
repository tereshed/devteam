import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message.dart';

/// Bubble user/assistant/system сообщения (Sprint 21 §8 frontend).
///
/// Markdown рендерится через `ChatMessage` с поддержкой fenced code, списков и т.д.
class AssistantMessageBubble extends StatelessWidget {
  const AssistantMessageBubble({super.key, required this.message});

  final AssistantMessageModel message;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AssistantMessageBubble',
    );
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    final isUser = message.isUser;
    final isSystem = message.isSystem;

    final (Color bg, Color fg) = switch (message.role) {
      assistantMessageRoleUser => (scheme.primaryContainer, scheme.onPrimaryContainer),
      assistantMessageRoleAssistant => (scheme.surfaceContainerHigh, scheme.onSurface),
      assistantMessageRoleSystem => (scheme.surfaceContainerLow, scheme.onSurfaceVariant),
      _ => (scheme.surfaceContainerHigh, scheme.onSurface),
    };

    final roleLabel = switch (message.role) {
      assistantMessageRoleUser => l10n.assistantMessageRoleUser,
      assistantMessageRoleAssistant => l10n.assistantMessageRoleAssistant,
      assistantMessageRoleSystem => l10n.assistantMessageRoleSystem,
      _ => message.role,
    };

    final chatRole = switch (message.role) {
      assistantMessageRoleUser => 'user',
      assistantMessageRoleAssistant => 'assistant',
      assistantMessageRoleSystem => 'system',
      _ => 'assistant',
    };

    return Align(
      alignment: isUser ? Alignment.centerRight : Alignment.centerLeft,
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 320),
        child: Container(
          margin: const EdgeInsets.symmetric(vertical: 4, horizontal: 12),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(12),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                roleLabel,
                style: theme.textTheme.labelSmall?.copyWith(
                  color: fg.withValues(alpha: 0.7),
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: 4),
              Theme(
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
              ),
            ],
          ),
        ),
      ),
    );
  }
}
