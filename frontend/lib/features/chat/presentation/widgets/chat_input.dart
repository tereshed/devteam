import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Intent отправки (Ctrl/Meta+Enter).
class _ChatInputSendIntent extends Intent {
  const _ChatInputSendIntent();
}

/// Поле ввода чата: многострочный ввод, вложения/стоп (опционально), отправка.
///
/// Не владеет [controller] и [focusNode] — [dispose] только у родителя.
/// Подписи кнопок ([sendTooltip], [stopTooltip], [attachTooltip]) в каноне ТЗ не перечислены;
/// передаются с экрана (l10n), опциональны — при `null` у [IconButton] не создаётся пустой [Tooltip].
class ChatInput extends StatelessWidget {
  const ChatInput({
    super.key,
    required this.controller,
    required this.focusNode,
    required this.onSend,
    this.onStop,
    this.onAttach,
    this.isStopActive = false,
    this.isSending = false,
    this.hintText,
    this.sendTooltip,
    this.stopTooltip,
    this.attachTooltip,
  });

  final TextEditingController controller;
  final FocusNode focusNode;

  /// Сырой [controller.text]; только если `trim` не пустой, `!isSending`, IME — см. [ChatInput].
  final ValueChanged<String> onSend;

  final VoidCallback? onStop;
  final VoidCallback? onAttach;

  /// Подсветка «идёт операция» для кнопки стоп.
  final bool isStopActive;

  /// Блокирует кнопку отправки и shortcut отправки.
  final bool isSending;

  /// Локализованный placeholder (передаёт родитель).
  final String? hintText;

  /// Подпись кнопки отправки ([IconButton.tooltip]); опционально — строку задаёт родитель (l10n).
  final String? sendTooltip;

  final String? stopTooltip;
  final String? attachTooltip;

  bool _maySendShortcut(TextEditingValue v) {
    if (isSending) {
      return false;
    }
    if (v.composing.isValid) {
      return false;
    }
    return v.text.trim().isNotEmpty;
  }

  void _dispatchSend() {
    if (isSending) {
      return;
    }
    final v = controller.value;
    if (v.composing.isValid) {
      return;
    }
    final t = controller.text;
    if (t.trim().isEmpty) {
      return;
    }
    onSend(t);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Shortcuts(
      shortcuts: const <ShortcutActivator, Intent>{
        SingleActivator(LogicalKeyboardKey.enter, control: true):
            _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.enter, meta: true): _ChatInputSendIntent(),
      },
      child: Actions(
        actions: <Type, Action<Intent>>{
          _ChatInputSendIntent: CallbackAction<_ChatInputSendIntent>(
            onInvoke: (_) {
              if (!_maySendShortcut(controller.value)) {
                return null;
              }
              _dispatchSend();
              return null;
            },
          ),
        },
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            if (onAttach != null) ...[
              IconButton(
                tooltip: attachTooltip,
                onPressed: onAttach,
                icon: const Icon(Icons.attach_file),
              ),
              const SizedBox(width: 8),
            ],
            Expanded(
              child: TextField(
                key: const ValueKey('chat_input_field'),
                controller: controller,
                focusNode: focusNode,
                minLines: 1,
                maxLines: 6,
                textInputAction: TextInputAction.newline,
                decoration: InputDecoration(
                  hintText: hintText,
                  border: const OutlineInputBorder(),
                ),
              ),
            ),
            const SizedBox(width: 8),
            if (onStop != null) ...[
              IconButton(
                tooltip: stopTooltip,
                onPressed: onStop,
                style: isStopActive
                    ? IconButton.styleFrom(
                        foregroundColor: theme.colorScheme.error,
                      )
                    : null,
                icon: const Icon(Icons.stop_circle_outlined),
              ),
              const SizedBox(width: 8),
            ],
            _ChatInputSendButton(
              controller: controller,
              isSending: isSending,
              tooltip: sendTooltip,
              onSend: _dispatchSend,
            ),
          ],
        ),
      ),
    );
  }
}

/// Перерисовка только кнопки отправки при изменении текста ([ValueListenableBuilder]).
class _ChatInputSendButton extends StatelessWidget {
  const _ChatInputSendButton({
    required this.controller,
    required this.isSending,
    this.tooltip,
    required this.onSend,
  });

  final TextEditingController controller;
  final bool isSending;
  final String? tooltip;
  final VoidCallback onSend;

  @override
  Widget build(BuildContext context) {
    return ValueListenableBuilder<TextEditingValue>(
      valueListenable: controller,
      builder: (context, value, _) {
        final canSend = value.text.trim().isNotEmpty && !isSending;
        return IconButton.filled(
          key: const ValueKey('chat_send_button'),
          onPressed: canSend ? onSend : null,
          tooltip: tooltip,
          icon: const Icon(Icons.send),
        );
      },
    );
  }
}
