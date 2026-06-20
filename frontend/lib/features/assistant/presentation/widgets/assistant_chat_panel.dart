import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_chat_controller.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_confirm_dialog.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_message_bubble.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_scope_badge.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_session_picker.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tool_call_card.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_input.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:go_router/go_router.dart';

/// Главная панель чата с ассистентом (Sprint 21 §10 frontend).
class AssistantChatPanel extends ConsumerStatefulWidget {
  const AssistantChatPanel({super.key});

  @override
  ConsumerState<AssistantChatPanel> createState() => _AssistantChatPanelState();
}

class _AssistantChatPanelState extends ConsumerState<AssistantChatPanel> {
  late final TextEditingController _inputController;
  late final FocusNode _inputFocus;
  late final ScrollController _scrollController;
  bool _confirmInFlight = false;

  @override
  void initState() {
    super.initState();
    _inputController = TextEditingController();
    _inputFocus = FocusNode();
    _scrollController = ScrollController();
    _scrollController.addListener(_maybeLoadOlder);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      // Лениво гарантируем наличие сессии при первом рендере. Ошибку
      // намеренно поглощаем — она уже отражена в `state.error`, виджет
      // покажет inline-сообщение с кнопкой Retry. unawaited+catchError
      // нужен, чтобы тестовая среда (отсутствие сети → 400) не валила
      // суплёт через unhandled future error.
      unawaited(
        ref
            .read(assistantChatControllerProvider.notifier)
            .ensureSession()
            .catchError((Object _) => ''),
      );
    });
  }

  @override
  void dispose() {
    _scrollController.removeListener(_maybeLoadOlder);
    _scrollController.dispose();
    _inputController.dispose();
    _inputFocus.dispose();
    super.dispose();
  }

  void _maybeLoadOlder() {
    // Reverse-список: maxScrollExtent — это самый «верх» истории.
    if (!_scrollController.hasClients) return;
    final pos = _scrollController.position;
    if (pos.pixels >= pos.maxScrollExtent - 80) {
      ref.read(assistantChatControllerProvider.notifier).loadOlder();
    }
  }

  Future<void> _onSend(String text) async {
    final controller = ref.read(assistantChatControllerProvider.notifier);
    _inputController.clear();
    await controller.sendMessage(text);
    if (!mounted) return;
    // Возвращаем фокус, чтобы можно было сразу печатать дальше.
    _inputFocus.requestFocus();
  }

  Future<void> _onConfirm(bool approved, String toolCallId) async {
    if (_confirmInFlight) return;
    setState(() => _confirmInFlight = true);
    try {
      await ref.read(assistantChatControllerProvider.notifier).confirmToolCall(
            toolCallId: toolCallId,
            approved: approved,
          );
    } finally {
      if (mounted) setState(() => _confirmInFlight = false);
    }
  }

  void _consumeNavigateIfAny(BuildContext context) {
    final pending =
        ref.read(assistantChatControllerProvider).pendingNavigate;
    if (pending == null) return;
    final route = pending.route;
    // Снимаем event ДО навигации — иначе при ребилде сработает повторно.
    ref.read(assistantChatControllerProvider.notifier).consumeNavigate();
    if (route.isEmpty) return;
    // Откладываем go() до postFrame: иначе мы навигируем из build/listener,
    // что часто ломает go_router внутренний state.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      context.go(route);
    });
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AssistantChatPanel');
    final state = ref.watch(assistantChatControllerProvider);

    final myAgentsAsync = ref.watch(myAgentsListProvider);
    final assistantAgent = myAgentsAsync.asData?.value.items
        .where((a) => a.role == 'assistant')
        .firstOrNull;
    final settings = assistantAgent?.settings ?? const {};
    var sttProvider = settings['stt_provider'] as String?;
    var sttModel = settings['stt_model'] as String?;

    if (sttProvider == null && sttModel != null && sttModel.isNotEmpty) {
      // Migrate old config on the fly
      if (sttModel == 'system') {
        sttProvider = 'system';
        sttModel = '';
      } else if (sttModel == 'whisper_openai') {
        sttProvider = 'openai';
        sttModel = 'whisper-1';
      } else if (sttModel == 'whisper_groq') {
        sttProvider = 'groq';
        sttModel = 'whisper-large-v3';
      } else if (sttModel == 'gemini_voice') {
        sttProvider = 'gemini';
        sttModel = 'gemini-1.5-flash';
      } else {
        sttProvider = 'system';
      }
    }

    final isVoiceEnabled = sttProvider != null && sttProvider.isNotEmpty && sttProvider != 'disabled';
    final voiceModel = sttProvider == 'system' ? 'system' : sttModel;

    // Listener-style: при появлении pendingNavigate выполняем context.go().
    ref.listen(
      assistantChatControllerProvider
          .select((s) => s.pendingNavigate?.route),
      (_, next) {
        if (next != null && next.isNotEmpty) {
          _consumeNavigateIfAny(context);
        }
      },
    );

    // При сбросе сессии (например, при смене проекта) гарантируем наличие новой сессии.
    ref.listen(
      assistantChatControllerProvider.select((s) => s.currentSessionId),
      (_, next) {
        if (next == null) {
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (!mounted) return;
            unawaited(
              ref
                  .read(assistantChatControllerProvider.notifier)
                  .ensureSession()
                  .catchError((Object _) => ''),
            );
          });
        }
      },
    );

    // Смена активного проекта (вход/выход/переключение) → ревалидируем scope
    // сессии. Контроллер keepAlive может не пересоздаться, поэтому полагаться
    // только на сброс currentSessionId нельзя: ensureSession сам проверит scope
    // текущей сессии и при mismatch переподберёт правильную (инцидент: чат
    // оставался глобальным внутри проекта).
    ref.listen(
      activeProjectIdProvider,
      (prev, next) {
        if (prev == next) return;
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (!mounted) return;
          unawaited(
            ref
                .read(assistantChatControllerProvider.notifier)
                .ensureSession()
                .catchError((Object _) => ''),
          );
        });
      },
    );

    final groups = groupAssistantMessages(state.messages);

    return Column(
      children: [
        // Header: session picker + busy hint.
        Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 8, 0),
          child: Row(
            children: [
              const Expanded(child: AssistantSessionPicker()),
              IconButton(
                tooltip: l10n.assistantNewSession,
                onPressed: state.creatingSession
                    ? null
                    : () => ref
                        .read(assistantChatControllerProvider.notifier)
                        .startNewSession(),
                icon: const Icon(Icons.add_comment_outlined),
              ),
            ],
          ),
        ),
        // Scope-бейдж: глобальный чат или имя проекта — переключение scope
        // при навигации видно явно, а не выглядит потерей контекста.
        const Padding(
          padding: EdgeInsets.fromLTRB(12, 4, 12, 0),
          child: Align(
            alignment: Alignment.centerLeft,
            child: AssistantScopeBadge(),
          ),
        ),
        if (state.isBusy)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            child: Row(
              children: [
                const SizedBox(
                  width: 14,
                  height: 14,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
                const SizedBox(width: 8),
                Text(
                  l10n.assistantSessionBusy,
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ],
            ),
          ),
        if (state.error != null)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
            child: Row(
              children: [
                Icon(Icons.error_outline,
                    size: 16,
                    color: Theme.of(context).colorScheme.error),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.assistantErrorGeneric,
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(
                          color: Theme.of(context).colorScheme.error,
                        ),
                  ),
                ),
                TextButton(
                  onPressed: () {
                    ref
                        .read(assistantChatControllerProvider.notifier)
                        .clearError();
                  },
                  child: Text(l10n.assistantRetry),
                ),
              ],
            ),
          ),
        const Divider(height: 1),
        Expanded(
          child: groups.isEmpty && !state.loadingHistory && state.pendingConfirm == null
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Text(
                      l10n.assistantEmptyChat,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                            color:
                                Theme.of(context).colorScheme.onSurfaceVariant,
                          ),
                    ),
                  ),
                )
              : _buildMessageList(context, state, groups),
        ),
        const Divider(height: 1),
        Padding(
          padding: const EdgeInsets.all(8),
          child: ChatInput(
            controller: _inputController,
            focusNode: _inputFocus,
            onSend: _onSend,
            onStop: state.isBusy
                ? () => ref
                    .read(assistantChatControllerProvider.notifier)
                    .stopSession()
                : null,
            isStopActive: state.isBusy,
            isSending: state.isBusy,
            hintText: l10n.assistantInputHint,
            sendTooltip: l10n.assistantSend,
            stopTooltip: l10n.assistantStop,
            isVoiceEnabled: isVoiceEnabled,
            voiceModel: voiceModel,
          ),
        ),
      ],
    );
  }

  Widget _buildMessageList(
    BuildContext context,
    AssistantChatState state,
    List<AssistantMessageGroup> groups,
  ) {
    // Items сверху вниз (по индексам в reverse-листе: 0 — самый низ).
    // Порядок снизу-вверх: [tail-loading, pendingConfirm, ...groups в reverse].
    final items = <Widget>[];
    if (state.pendingConfirm != null) {
      final ev = state.pendingConfirm!;
      items.add(
        AssistantConfirmDialog(
          event: ev,
          busy: _confirmInFlight,
          onApprove: () => _onConfirm(true, ev.toolCallId),
          onDeny: () => _onConfirm(false, ev.toolCallId),
        ),
      );
    }
    for (final g in groups.reversed) {
      if (g.isToolCall) {
        items.add(
          AssistantToolCallCard(
            assistantMessage: g.assistantMessage,
            toolResult: g.toolResult,
          ),
        );
      } else {
        items.add(AssistantMessageBubble(message: g.assistantMessage));
      }
    }
    if (state.hasMore) {
      items.add(
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 8),
          child: Center(
            child: state.loadingHistory
                ? const SizedBox(
                    width: 18,
                    height: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : TextButton(
                    onPressed: () => ref
                        .read(assistantChatControllerProvider.notifier)
                        .loadOlder(),
                    child: Text(requireAppLocalizations(context,
                            where: 'AssistantChatPanel._buildMessageList')
                        .assistantLoadOlder),
                  ),
          ),
        ),
      );
    }
    return ListView.builder(
      controller: _scrollController,
      reverse: true,
      padding: const EdgeInsets.symmetric(vertical: 4),
      itemCount: items.length,
      itemBuilder: (context, index) => items[index],
    );
  }
}
