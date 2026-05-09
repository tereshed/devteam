import 'dart:async';
import 'dart:developer' as developer;

import 'package:flutter/foundation.dart' show setEquals;
import 'package:flutter/material.dart';
import 'package:flutter/scheduler.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/domain/linked_task_snapshots.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/presentation/controllers/chat_controller.dart';
import 'package:frontend/features/chat/presentation/state/chat_state.dart';
import 'package:frontend/features/chat/presentation/state/pending_message.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_input.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_card.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_visuals.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Отступ между пузырём [ChatMessage] и блоком [TaskStatusCard] (ТЗ 11.7).
const double _kBubbleToCardGap = 8;

/// Сколько раз ждать [SchedulerBinding.instance.endOfFrame] перед fallback при cross-branch push
/// (чат → деталь задачи). Должно совпадать с циклом в [_pushTaskDetailWhenTasksNavigatorReady];
/// виджет-тест «race guard» использует то же число [WidgetTester.pump].
const int kTasksCrossBranchPushMaxRetries = 3;

/// Открытие детали задачи из чата: переключить shell на ветку **tasks**, затем
/// `push` с контекстом [projectDashboardShellTasksNavigatorKey] — иначе URL уходит в
/// chat-navigator и drift вкладки/стека (12.5 review ит.4).
///
/// [StatefulNavigationShell.goBranch] с [initialLocation]=**true** сбрасывает стек ветки tasks к
/// `/projects/:id/tasks` перед push детали: **Back** с экрана задачи ведёт на **список**, а не на
/// предыдущую задачу в стеке этой вкладки (контракт 12.5 review ит.5 п.2).
///
/// **Trade-off (review ит.6):** если пользователь был на `/tasks/A`, перешёл в чат и открыл карточку
/// задачи **B**, стек **A** сбрасывается — после **Back** с **B** он попадает на список, а не на **A**.
/// Сохранение **A** потребовало бы [initialLocation]=false и вернуло бы проблему «Back → A вместо
/// списка» (ит.5 п.2). Гибрид «как подзадачи» возможен позже при явном продуктовом запросе.
void _openTaskDetailFromChatShell(
  BuildContext context,
  String projectId,
  String taskId,
) {
  final shell = StatefulNavigationShell.maybeOf(context);
  final location = '/projects/$projectId/tasks/$taskId';
  if (shell != null) {
    final tasksIdx = projectDashboardShellBranchPaths
        .indexOf(projectDashboardShellBranchTasksSegment);
    if (tasksIdx >= 0) {
      shell.goBranch(tasksIdx, initialLocation: true);
      unawaited(_pushTaskDetailWhenTasksNavigatorReady(context, location));
      return;
    }
  }
  context.push(location);
}

/// После [goBranch] контекст tasks-[Navigator] может появиться только со следующего кадра.
/// Ждём [kTasksCrossBranchPushMaxRetries] раз [SchedulerBinding.instance.endOfFrame].
Future<void> _pushTaskDetailWhenTasksNavigatorReady(
  BuildContext context,
  String location,
) async {
  for (var attempt = 0; attempt < kTasksCrossBranchPushMaxRetries; attempt++) {
    await SchedulerBinding.instance.endOfFrame;
    final tasksCtx = projectDashboardShellTasksNavigatorKey.currentContext;
    if (tasksCtx != null && tasksCtx.mounted) {
      GoRouter.of(tasksCtx).push(location);
      return;
    }
  }
  developer.log(
    'tasks navigator context still null after goBranch (fallback push via chat context — possible shell/tab drift)',
    name: 'devteam.chat.cross_branch',
    level: 900,
  );
  if (context.mounted) {
    context.push(location);
  }
}

Widget _messageTaskStatusCard(
  BuildContext context,
  String projectId,
  ConversationMessageModel message,
  String taskId,
) {
  final snap = linkedTaskSnapshotForMessage(message, taskId);
  return TaskStatusCard(
    key: ValueKey<String>(taskId),
    taskId: taskId,
    title: snap.title,
    status: snap.status,
    errorMessage: snap.errorMessage,
    agentRole: taskCardAgentRoleTryParse(snap.agentRoleRaw),
    onOpen: (id) => _openTaskDetailFromChatShell(context, projectId, id),
  );
}

/// Константы прокрутки чата ([ListView.reverse] = true: низ — [ScrollPosition.pixels] → 0).
abstract final class ChatScreenScroll {
  /// Гард «у низа» для auto-scroll (logical px).
  static const double bottomStickPx = 36;

  /// Порог от верха (визуально старые сообщения) для [ChatController.loadOlder].
  static const double loadOlderLeadPx = 96;

  /// Длительность [ScrollController.animateTo] к низу ленты ([ListView.reverse]).
  static const Duration scrollToBottomDuration = Duration(milliseconds: 220);
}

/// Экран чата: история, ввод, отправка, пагинация вверх, pending/retry.
class ChatScreen extends ConsumerStatefulWidget {
  const ChatScreen({
    super.key,
    required this.projectId,
    required this.conversationId,
  });

  final String projectId;
  final String conversationId;

  @override
  ConsumerState<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends ConsumerState<ChatScreen> {
  final ScrollController _scrollController = ScrollController();
  final TextEditingController _textController = TextEditingController();
  final FocusNode _inputFocus = FocusNode();

  /// In-flight [ChatController.send]: блок [ChatInput.isSending] и идемпотентность UI.
  bool _sendInFlight = false;

  bool _userAtBottom = true;
  bool _didInitialBottomScroll = false;
  /// Число активных цепочек [ScrollController.animateTo](0) — при серии апдейтов не сбрасывать
  /// «программный скролл» по первому whenComplete отменённой анимации (см. ревью n1).
  int _programmaticScrollDepth = 0;

  ChatController get _notifier => ref.read(
        chatControllerProvider(
          projectId: widget.projectId,
          conversationId: widget.conversationId,
        ).notifier,
      );

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScrollTick);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScrollTick);
    _scrollController.dispose();
    _textController.dispose();
    _inputFocus.dispose();
    super.dispose();
  }

  void _handleUserScrollInterruptAnimation() {
    // Только пока идёт programmatic animateTo к низу — иначе jumpTo ломает drag (в т.ч. widget-тесты).
    // Вложенный скролл из 11.6/11.7 — ловим здесь, у списка сообщений (не у всего AdaptiveContainer).
    // При стриме ассистента (11.9) при конфликте со скроллом — пересмотреть здесь первым делом.
    if (_programmaticScrollDepth == 0) {
      return;
    }
    if (_scrollController.hasClients) {
      _scrollController.jumpTo(_scrollController.offset);
    }
  }

  void _onScrollTick() {
    if (!_scrollController.hasClients) {
      return;
    }
    final position = _scrollController.position;
    final pixels = position.pixels;
    _userAtBottom = pixels <= ChatScreenScroll.bottomStickPx;

    final asyncChat = ref.read(
      chatControllerProvider(
        projectId: widget.projectId,
        conversationId: widget.conversationId,
      ),
    );
    final v = asyncChat.maybeWhen(
      data: (s) => s,
      orElse: () => null,
    );
    if (v == null ||
        !v.hasMoreOlder ||
        v.isLoadingOlder ||
        v.isLoadingInitial) {
      return;
    }

    final max = position.maxScrollExtent;
    if (pixels >= max - ChatScreenScroll.loadOlderLeadPx) {
      unawaited(
        _loadOlderWithFeedback(),
      );
    }
  }

  /// Пагинация: ошибка API не сносит ленту, но даёт SnackBar (без `unawaited` → необработанный error).
  Future<void> _loadOlderWithFeedback() async {
    if (!mounted) {
      return;
    }
    final l10n = AppLocalizations.of(context)!;
    try {
      await _notifier.loadOlder();
    } catch (e) {
      if (!mounted) {
        return;
      }
      if (e is ConversationCancelledException) {
        return;
      }
      ScaffoldMessenger.maybeOf(context)?.showSnackBar(
        SnackBar(content: Text(chatErrorTitle(l10n, e))),
      );
    }
  }

  void _scheduleScrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      if (!_scrollController.hasClients) {
        return;
      }
      _programmaticScrollDepth++;
      _scrollController
          .animateTo(
        0,
        duration: ChatScreenScroll.scrollToBottomDuration,
        curve: Curves.easeOutCubic,
      )
          .whenComplete(() {
        if (!mounted) {
          return;
        }
        _programmaticScrollDepth--;
        if (_programmaticScrollDepth < 0) {
          _programmaticScrollDepth = 0;
        }
        if (_programmaticScrollDepth == 0) {
          _didInitialBottomScroll = true;
        }
      });
    });
  }

  void _maybeAutoScroll(AsyncValue<ChatState>? previous, AsyncValue<ChatState> next) {
    if (!next.hasValue) {
      return;
    }
    final cur = next.requireValue;
    final prev = previous?.maybeWhen(
      data: (s) => s,
      orElse: () => null,
    );

    final msgGrew =
        prev != null && cur.messages.length > prev.messages.length;
    final initialReady =
        prev?.isLoadingInitial == true && !cur.isLoadingInitial;
    final pendingKeysChanged = prev == null ||
        !setEquals(
          cur.pendingByClientId.keys.toSet(),
          prev.pendingByClientId.keys.toSet(),
        );

    final trigger =
        msgGrew || initialReady || pendingKeysChanged;
    if (!trigger) {
      return;
    }

    final allowScroll =
        _userAtBottom || (!_didInitialBottomScroll && !cur.isLoadingInitial);
    if (!allowScroll) {
      return;
    }

    _scheduleScrollToBottom();
  }

  Future<void> _submitText(String raw) async {
    if (raw.trim().isEmpty || _sendInFlight) {
      return;
    }
    setState(() => _sendInFlight = true);
    try {
      final outcome = await _notifier.send(raw);
      if (outcome == ChatSendOutcome.completed && mounted) {
        _textController.clear();
        _inputFocus.requestFocus();
      }
    } catch (_) {
      // Фатальная ошибка — [AsyncError] на провайдере; SnackBar не дублируем.
    } finally {
      if (mounted) {
        setState(() => _sendInFlight = false);
      } else {
        _sendInFlight = false;
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final asyncChat = ref.watch(
      chatControllerProvider(
        projectId: widget.projectId,
        conversationId: widget.conversationId,
      ),
    );

    ref.listen(
      chatControllerProvider(
        projectId: widget.projectId,
        conversationId: widget.conversationId,
      ),
      _maybeAutoScroll,
    );

    final stale = asyncChat.value;
    final chatState = stale ?? ChatState.initial();
    final showInitialSpinner =
        chatState.isLoadingInitial && chatState.messages.isEmpty;
    final isReloadWithData = asyncChat.isLoading && stale != null;

    if (asyncChat.hasError && asyncChat.error is ConversationNotFoundException) {
      return Scaffold(
        appBar: AppBar(
          title: Text(l10n.chatScreenAppBarFallbackTitle),
        ),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  l10n.chatErrorConversationNotFound,
                  textAlign: TextAlign.center,
                  style: theme.textTheme.titleMedium,
                ),
                const SizedBox(height: 16),
                // TODO(11.x): симметрия с 12.5 — к списку задач проекта
                // (`/projects/$projectId/tasks`), не глобальный `/projects` (review 12.5 ит.4).
                FilledButton(
                  onPressed: () => context.go('/projects'),
                  child: Text(l10n.chatScreenNotFoundBack),
                ),
              ],
            ),
          ),
        ),
      );
    }

    if (asyncChat.hasError && !isReloadWithData) {
      final err = asyncChat.error!;
      return Scaffold(
        appBar: AppBar(title: Text(l10n.chatScreenAppBarFallbackTitle)),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  chatErrorTitle(l10n, err),
                  textAlign: TextAlign.center,
                  style: theme.textTheme.titleMedium,
                ),
                if (chatErrorDetail(err) != null) ...[
                  const SizedBox(height: 8),
                  Text(
                    chatErrorDetail(err)!,
                    textAlign: TextAlign.center,
                    style: theme.textTheme.bodySmall,
                  ),
                ],
                const SizedBox(height: 16),
                FilledButton(
                  onPressed: () => ref.invalidate(
                    chatControllerProvider(
                      projectId: widget.projectId,
                      conversationId: widget.conversationId,
                    ),
                  ),
                  child: Text(l10n.retry),
                ),
              ],
            ),
          ),
        ),
      );
    }

    final rawTitle = chatState.conversation?.title;
    final title = (rawTitle == null || rawTitle.trim().isEmpty)
        ? l10n.chatScreenAppBarFallbackTitle
        : rawTitle;

    return Scaffold(
      resizeToAvoidBottomInset: true,
      appBar: AppBar(
        title: Text(title),
      ),
      body: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (isReloadWithData)
            const LinearProgressIndicator(minHeight: 2),
          Expanded(
            child: showInitialSpinner
                ? const Center(child: CircularProgressIndicator())
                : AdaptiveContainer(
                    child: _ChatMessageList(
                      projectId: widget.projectId,
                      scrollController: _scrollController,
                      chatState: chatState,
                      theme: theme,
                      l10n: l10n,
                      onRetryPending: (id) => unawaited(
                        ref
                            .read(
                              chatControllerProvider(
                                projectId: widget.projectId,
                                conversationId: widget.conversationId,
                              ).notifier,
                            )
                            .retrySend(id),
                      ),
                      onUserScrollInterruptAnimation:
                          _handleUserScrollInterruptAnimation,
                    ),
                  ),
          ),
          SafeArea(
            top: false,
            child: Material(
              elevation: 2,
              color: theme.colorScheme.surface,
              child: AdaptiveContainer(
                usePadding: true,
                child: ChatInput(
                  controller: _textController,
                  focusNode: _inputFocus,
                  onSend: (text) => unawaited(_submitText(text)),
                  isSending: _sendInFlight,
                  hintText: l10n.chatInputHint,
                  sendTooltip: l10n.chatInputSendTooltip,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _ChatMessageList extends StatelessWidget {
  const _ChatMessageList({
    required this.projectId,
    required this.scrollController,
    required this.chatState,
    required this.theme,
    required this.l10n,
    required this.onRetryPending,
    required this.onUserScrollInterruptAnimation,
  });

  final String projectId;
  final ScrollController scrollController;
  final ChatState chatState;
  final ThemeData theme;
  final AppLocalizations l10n;
  final void Function(String clientMessageId) onRetryPending;
  final VoidCallback onUserScrollInterruptAnimation;

  @override
  Widget build(BuildContext context) {
    // DESC по lastAttemptAt: индекс 0 при reverse:true — визуальный низ; два быстрых send → новее ниже в списке индексов (= у низа экрана).
    final pendingSorted = chatState.pendingByClientId.values.toList()
      ..sort((a, b) => b.lastAttemptAt.compareTo(a.lastAttemptAt));

    final msgs = chatState.messages;
    final showLoader = chatState.isLoadingOlder;
    final pendingCount = pendingSorted.length;
    final msgCount = msgs.length;
    final itemCount = pendingCount + msgCount + (showLoader ? 1 : 0);

    return NotificationListener<UserScrollNotification>(
      onNotification: (_) {
        onUserScrollInterruptAnimation();
        return false;
      },
      child: ListView.builder(
        key: const ValueKey('chat_message_list'),
        controller: scrollController,
        reverse: true,
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 12),
        itemCount: itemCount,
        itemBuilder: (context, i) {
          if (i < pendingCount) {
            final p = pendingSorted[i];
            return _PendingBubble(
              pending: p,
              theme: theme,
              l10n: l10n,
              onRetry: () => onRetryPending(p.clientMessageId),
            );
          }
          final j = i - pendingCount;
          if (j < msgCount) {
            final m = msgs[msgCount - 1 - j];
            return _MessageBubble(
              projectId: projectId,
              message: m,
              theme: theme,
              l10n: l10n,
            );
          }
          return const _LoadOlderIndicator();
        },
      ),
    );
  }
}

class _LoadOlderIndicator extends StatelessWidget {
  const _LoadOlderIndicator();

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 12),
      child: Center(
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
            const SizedBox(width: 12),
            Text(
              l10n.chatScreenLoadingOlder,
              style: theme.textTheme.bodySmall,
            ),
          ],
        ),
      ),
    );
  }
}

bool _metadataStreamingAssistant(Map<String, dynamic>? meta) {
  if (meta == null) {
    return false;
  }
  // Канонический ключ после [ConversationMessageModel.fromJson] (`streaming`; см. модель).
  return meta['streaming'] == true;
}

class _MessageBubble extends StatelessWidget {
  const _MessageBubble({
    required this.projectId,
    required this.message,
    required this.theme,
    required this.l10n,
  });

  final String projectId;
  final ConversationMessageModel message;
  final ThemeData theme;
  final AppLocalizations l10n;

  @override
  Widget build(BuildContext context) {
    final label = _semanticLabel(l10n, message);
    final isStreamingBody =
        message.role == 'assistant' && _metadataStreamingAssistant(message.metadata);
    final isUser = message.role == 'user';
    final isAssistant = message.role == 'assistant';

    Color bg;
    Alignment align;
    if (isUser) {
      bg = theme.colorScheme.primaryContainer;
      align = Alignment.centerRight;
    } else if (isAssistant) {
      bg = theme.colorScheme.surfaceContainerHighest;
      align = Alignment.centerLeft;
    } else {
      bg = theme.colorScheme.surfaceContainerLow;
      align = Alignment.center;
    }

    final bubble = Container(
      margin: const EdgeInsets.symmetric(vertical: 4),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(12),
      ),
      child: ChatMessage(
        key: ValueKey<String>('conv_msg_${message.id}'),
        role: message.role,
        content: message.content,
        isStreaming: isStreamingBody,
      ),
    );

    final linked = message.linkedTaskIds;
    final cross = switch (align) {
      Alignment.centerRight => CrossAxisAlignment.end,
      Alignment.centerLeft => CrossAxisAlignment.start,
      _ => CrossAxisAlignment.center,
    };

    final cardColumn = <Widget>[
      bubble,
      if (linked.isNotEmpty) ...[
        const SizedBox(height: _kBubbleToCardGap),
        for (var i = 0; i < linked.length; i++) ...[
          if (i > 0) const SizedBox(height: 8),
          _messageTaskStatusCard(context, projectId, message, linked[i]),
        ],
      ],
    ];

    // 11.6: во время стрима не включаем liveRegion — иначе каждый чанк озвучивается заново.
    return Semantics(
      label: label,
      liveRegion: false,
      child: Align(
        alignment: align,
        child: ConstrainedBox(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.sizeOf(context).width * 0.88,
          ),
          child: Column(
            crossAxisAlignment: cross,
            mainAxisSize: MainAxisSize.min,
            children: cardColumn,
          ),
        ),
      ),
    );
  }
}

String _semanticSample(String content) {
  const maxLen = 240;
  if (content.length <= maxLen) {
    return content;
  }
  return '${content.substring(0, maxLen)}…';
}

String _semanticLabel(AppLocalizations l10n, ConversationMessageModel m) {
  final sample = _semanticSample(m.content);
  return switch (m.role) {
    'user' => l10n.chatScreenMessageSemanticUser(sample),
    'assistant' => l10n.chatScreenMessageSemanticAssistant(sample),
    _ => l10n.chatScreenMessageSemanticSystem(sample),
  };
}

class _PendingBubble extends StatelessWidget {
  const _PendingBubble({
    required this.pending,
    required this.theme,
    required this.l10n,
    required this.onRetry,
  });

  final PendingMessage pending;
  final ThemeData theme;
  final AppLocalizations l10n;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final err = pending.lastError != null;
    return Align(
      alignment: Alignment.centerRight,
      child: ConstrainedBox(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * 0.88,
        ),
        child: Semantics(
          label: l10n.chatScreenMessageSemanticUser(
            _semanticSample(pending.content),
          ),
          child: Container(
            margin: const EdgeInsets.symmetric(vertical: 4),
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(12),
              border: Border.all(
                color: err ? theme.colorScheme.error : theme.colorScheme.outline,
              ),
              color: theme.colorScheme.primaryContainer.withValues(alpha: 0.6),
            ),
            child: RepaintBoundary(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  ChatMessage(
                    key: ValueKey<String>('pending_msg_${pending.clientMessageId}'),
                    role: 'user',
                    content: pending.content,
                    isStreaming: false,
                  ),
                  const SizedBox(height: 6),
                  if (!err)
                    Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const SizedBox(
                          width: 14,
                          height: 14,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                        const SizedBox(width: 8),
                        Text(
                          l10n.chatScreenPendingSending,
                          style: theme.textTheme.bodySmall,
                        ),
                      ],
                    )
                  else
                    TextButton.icon(
                      key: ValueKey('pending_retry_${pending.clientMessageId}'),
                      onPressed: onRetry,
                      icon: const Icon(Icons.refresh, size: 18),
                      label: Text(l10n.chatScreenPendingRetry),
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
