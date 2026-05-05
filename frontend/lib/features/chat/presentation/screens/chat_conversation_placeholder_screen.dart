import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Ветка `/projects/:id/chat` без `:conversationId` — до экрана списка бесед (Sprint 11+).
class ChatConversationPlaceholderScreen extends StatelessWidget {
  const ChatConversationPlaceholderScreen({
    super.key,
    required this.projectId,
  });

  final String projectId;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    return Center(
      key: ValueKey('chat-placeholder-$projectId'),
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Text(
          l10n.chatScreenSelectConversationHint,
          textAlign: TextAlign.center,
        ),
      ),
    );
  }
}
