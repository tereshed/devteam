import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Ветка `/projects/:id/chat` без `:conversationId` — до экрана списка бесед (Sprint 11+).
/// Загружает список чатов, автоматически перенаправляет на первый чат или создает дефолтный "General".
class ChatConversationPlaceholderScreen extends ConsumerStatefulWidget {
  const ChatConversationPlaceholderScreen({
    super.key,
    required this.projectId,
  });

  final String projectId;

  @override
  ConsumerState<ChatConversationPlaceholderScreen> createState() =>
      _ChatConversationPlaceholderScreenState();
}

class _ChatConversationPlaceholderScreenState
    extends ConsumerState<ChatConversationPlaceholderScreen> {
  bool _isLoading = false;
  Object? _error;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadAndRedirect);
  }

  @override
  void didUpdateWidget(covariant ChatConversationPlaceholderScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.projectId != widget.projectId) {
      _loadAndRedirect();
    }
  }

  Future<void> _loadAndRedirect() async {
    if (!mounted) return;
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final repo = ref.read(conversationRepositoryProvider);
      final response = await repo.listConversations(widget.projectId);
      if (!mounted) return;

      if (response.conversations.isNotEmpty) {
        final conversationId = response.conversations.first.id;
        final target = '/projects/${widget.projectId}/chat/$conversationId';
        context.replace(target);
      } else {
        final newConv = await repo.createConversation(
          widget.projectId,
          const CreateConversationRequest(title: 'General'),
        );
        if (!mounted) return;
        final target = '/projects/${widget.projectId}/chat/${newConv.id}';
        context.replace(target);
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
      });
    } finally {
      if (mounted) {
        setState(() {
          _isLoading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;

    if (_error != null) {
      return Scaffold(
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: DataLoadErrorMessage(
              title: l10n.dataLoadError,
              actionLabel: l10n.retry,
              onAction: _loadAndRedirect,
            ),
          ),
        ),
      );
    }

    return Scaffold(
      key: ValueKey('chat-placeholder-${widget.projectId}'),
      body: const Center(
        child: Padding(
          padding: EdgeInsets.all(24),
          child: CircularProgressIndicator(),
        ),
      ),
    );
  }
}

