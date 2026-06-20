import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:record/record.dart';

/// Intent отправки (Enter, Ctrl+Enter, Meta+Enter).
class _ChatInputSendIntent extends Intent {
  const _ChatInputSendIntent();
}

/// Intent голосового ввода (Alt+V).
class _ChatInputVoiceIntent extends Intent {
  const _ChatInputVoiceIntent();
}

/// Поле ввода чата: многострочный ввод, голосовой ввод, отправка.
///
/// Не владеет [controller] и [focusNode] — [dispose] только у родителя.
class ChatInput extends ConsumerStatefulWidget {
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
    this.isVoiceEnabled = false,
    this.voiceModel,
  });

  final TextEditingController controller;
  final FocusNode focusNode;

  /// Сырой [controller.text]; только если `trim` не пустой, `!isSending`
  final ValueChanged<String> onSend;

  final VoidCallback? onStop;
  final VoidCallback? onAttach;

  /// Подсветка «идёт операция» для кнопки стоп.
  final bool isStopActive;

  /// Блокирует кнопку отправки и shortcut отправки.
  final bool isSending;

  /// Локализованный placeholder (передаёт родитель).
  final String? hintText;

  /// Подпись кнопки отправки ([IconButton.tooltip]).
  final String? sendTooltip;

  final String? stopTooltip;
  final String? attachTooltip;

  /// Активен ли голосовой ввод (настроена ли модель).
  final bool isVoiceEnabled;

  /// Выбранная модель распознавания (STT).
  final String? voiceModel;

  @override
  ConsumerState<ChatInput> createState() => _ChatInputState();
}

class _ChatInputState extends ConsumerState<ChatInput> with SingleTickerProviderStateMixin {
  final _audioRecorder = AudioRecorder();
  bool _isRecording = false;
  bool _blink = false;
  File? _currentAudioFile;
  Timer? _recordingTimer;
  int _recordingSeconds = 0;
  AnimationController? _waveController;
  bool _isTranscribing = false;

  @override
  void initState() {
    super.initState();
    _waveController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1000),
    );
  }

  @override
  void dispose() {
    _recordingTimer?.cancel();
    _audioRecorder.dispose();
    _waveController?.dispose();
    _cleanupVoiceFile();
    super.dispose();
  }

  String _formatDuration(int seconds) {
    final min = seconds ~/ 60;
    final sec = seconds % 60;
    return '$min:${sec.toString().padLeft(2, '0')}';
  }

  void _cleanupVoiceFile() {
    if (_currentAudioFile != null) {
      final fileToDelete = _currentAudioFile!;
      _currentAudioFile = null;
      if (!kIsWeb) {
        try {
          if (fileToDelete.existsSync()) {
            fileToDelete.deleteSync();
          }
        } catch (_) {}
      }
    }
  }

  Future<void> _toggleRecording() async {
    if (_isRecording) {
      await _stopRecording(save: true);
    } else {
      await _startRecording();
    }
  }

  Future<void> _startRecording() async {
    if (!widget.isVoiceEnabled || _isTranscribing) return;

    try {
      if (!await _audioRecorder.hasPermission()) {
        return;
      }

      final tempDir = Directory.systemTemp;
      final path = '${tempDir.path}/devteam_voice_${DateTime.now().millisecondsSinceEpoch}.m4a';

      await _audioRecorder.start(
        const RecordConfig(encoder: AudioEncoder.aacLc),
        path: path,
      );

      _currentAudioFile = File(path);

      _waveController?.repeat(reverse: true);

      setState(() {
        _isRecording = true;
        _recordingSeconds = 0;
        _blink = true;
      });

      _recordingTimer = Timer.periodic(const Duration(seconds: 1), (timer) {
        if (mounted) {
          setState(() {
            _recordingSeconds++;
            _blink = !_blink;
          });
        }
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Ошибка при старте записи: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    }
  }

  Future<void> _stopRecording({required bool save}) async {
    _recordingTimer?.cancel();
    _recordingTimer = null;
    _waveController?.stop();

    if (!mounted) return;

    final wasRecording = _isRecording;
    setState(() {
      _isRecording = false;
    });

    if (!wasRecording) return;

    try {
      final path = await _audioRecorder.stop();
      if (save && path != null && widget.voiceModel != null) {
        final file = File(path);
        if (file.existsSync()) {
          setState(() {
            _isTranscribing = true;
          });

          final bytes = await file.readAsBytes();
          final filename = path.split('/').last;

          final repo = ref.read(assistantRepositoryProvider);
          final recognizedText = await repo.transcribe(
            bytes: bytes,
            filename: filename,
          );

          if (mounted) {
            final originalText = widget.controller.text;
            final separator = originalText.isEmpty || originalText.endsWith('\n') ? '' : ' ';
            final fullText = '$originalText$separator$recognizedText';
            widget.controller.text = fullText;
            widget.controller.selection = TextSelection.collapsed(offset: fullText.length);

            setState(() {
              _isTranscribing = false;
            });

            if (fullText.trim().isNotEmpty) {
              _dispatchSend();
            }
          }
        }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Ошибка распознавания речи: $e'),
            backgroundColor: Colors.red,
          ),
        );
      }
    } finally {
      if (mounted) {
        setState(() {
          _isTranscribing = false;
        });
      }
      _cleanupVoiceFile();
    }
  }

  bool _maySendShortcut(TextEditingValue v) {
    if (widget.isSending || _isTranscribing) {
      return false;
    }
    if (v.composing.isValid) {
      return false;
    }
    return v.text.trim().isNotEmpty;
  }

  void _dispatchSend() {
    if (widget.isSending || _isTranscribing) {
      return;
    }
    final v = widget.controller.value;
    if (v.composing.isValid) {
      return;
    }
    final t = widget.controller.text;
    if (t.trim().isEmpty) {
      return;
    }
    widget.onSend(t);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final l10n = AppLocalizations.of(context);

    // Локализованные тексты для подсказок
    final voiceTooltip = widget.isVoiceEnabled
        ? (l10n?.chatInputVoiceTooltip ?? 'Голосовой ввод (Alt+V)')
        : (l10n?.chatInputVoiceDisabledTooltip ??
            'Голосовой ввод не активен (настройте модель в настройках)');
    final recordHint = _isRecording
        ? (l10n?.chatInputVoiceRecordingHint(_recordingSeconds) ??
            'Идет запись... Говорите ($_recordingSecondsс). Нажмите Alt+V для завершения')
        : widget.hintText;

    return Shortcuts(
      shortcuts: const <ShortcutActivator, Intent>{
        SingleActivator(LogicalKeyboardKey.enter): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.numpadEnter): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.enter, control: true): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.enter, meta: true): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.numpadEnter, control: true): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.numpadEnter, meta: true): _ChatInputSendIntent(),
        SingleActivator(LogicalKeyboardKey.keyV, alt: true): _ChatInputVoiceIntent(),
      },
      child: Actions(
        actions: <Type, Action<Intent>>{
          _ChatInputSendIntent: CallbackAction<_ChatInputSendIntent>(
            onInvoke: (_) {
              if (!_maySendShortcut(widget.controller.value)) {
                return null;
              }
              Future.microtask(_dispatchSend);
              return null;
            },
          ),
          _ChatInputVoiceIntent: CallbackAction<_ChatInputVoiceIntent>(
            onInvoke: (_) {
              if (widget.isVoiceEnabled) {
                _toggleRecording();
              }
              return null;
            },
          ),
        },
        child: _isRecording
            ? Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  Stack(
                    alignment: Alignment.center,
                    children: [
                      AnimatedBuilder(
                        animation: _waveController!,
                        builder: (context, child) {
                          final double scale = 1.0 + 0.6 * _waveController!.value;
                          final double opacity = 1.0 - _waveController!.value;
                          return Container(
                            width: 40 * scale,
                            height: 40 * scale,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: Colors.red.withValues(alpha: opacity * 0.4),
                            ),
                          );
                        },
                      ),
                      IconButton(
                        key: const ValueKey('chat_voice_button'),
                        icon: const Icon(Icons.mic, color: Colors.white),
                        style: IconButton.styleFrom(
                          backgroundColor: Colors.red,
                        ),
                        tooltip: voiceTooltip,
                        onPressed: () => _stopRecording(save: true),
                      ),
                    ],
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Container(
                      height: 48,
                      padding: const EdgeInsets.symmetric(horizontal: 12),
                      decoration: BoxDecoration(
                        color: theme.colorScheme.errorContainer.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(24),
                        border: Border.all(
                          color: theme.colorScheme.error.withValues(alpha: 0.24),
                        ),
                      ),
                      child: Row(
                        children: [
                          AnimatedBuilder(
                            animation: _waveController!,
                            builder: (context, child) {
                              final double opacity = 0.3 + 0.7 * (1.0 - _waveController!.value);
                              return Icon(
                                Icons.circle,
                                color: Colors.red.withValues(alpha: opacity),
                                size: 10,
                              );
                            },
                          ),
                          const SizedBox(width: 8),
                          Text(
                            'Запись: ${_formatDuration(_recordingSeconds)}',
                            style: theme.textTheme.bodyMedium?.copyWith(
                              fontWeight: FontWeight.w600,
                              color: theme.colorScheme.onErrorContainer,
                            ),
                          ),
                          const Spacer(),
                          Row(
                            mainAxisSize: MainAxisSize.min,
                            children: List.generate(6, (index) {
                              return AnimatedBuilder(
                                animation: _waveController!,
                                builder: (context, child) {
                                  final value = (index * 0.15 + _waveController!.value) % 1.0;
                                  final double scale = 0.2 + 0.8 * (value - 0.5).abs() * 2;
                                  return Container(
                                    width: 3,
                                    height: 18 * scale,
                                    margin: const EdgeInsets.symmetric(horizontal: 1.5),
                                    decoration: BoxDecoration(
                                      color: theme.colorScheme.error,
                                      borderRadius: BorderRadius.circular(1.5),
                                    ),
                                  );
                                },
                              );
                            }),
                          ),
                          const SizedBox(width: 8),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  IconButton(
                    tooltip: 'Отмена',
                    icon: Icon(Icons.delete_outline, color: theme.colorScheme.error),
                    onPressed: () => _stopRecording(save: false),
                  ),
                  const SizedBox(width: 8),
                  IconButton.filled(
                    tooltip: 'Готово',
                    icon: const Icon(Icons.check),
                    onPressed: () => _stopRecording(save: true),
                    style: IconButton.styleFrom(
                      backgroundColor: Colors.green,
                    ),
                  ),
                ],
              )
            : Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  if (widget.onAttach != null) ...[
                    IconButton(
                      tooltip: widget.attachTooltip,
                      onPressed: widget.isSending || _isTranscribing ? null : widget.onAttach,
                      icon: const Icon(Icons.attach_file),
                    ),
                    const SizedBox(width: 8),
                  ],
                  IconButton(
                    key: const ValueKey('chat_voice_button'),
                    icon: Icon(
                      widget.isVoiceEnabled ? Icons.mic_none : Icons.mic_off,
                      color: widget.isVoiceEnabled && !widget.isSending && !_isTranscribing
                          ? null
                          : theme.colorScheme.onSurface.withValues(alpha: 0.38),
                    ),
                    tooltip: voiceTooltip,
                    onPressed: widget.isVoiceEnabled && !widget.isSending && !_isTranscribing
                        ? _toggleRecording
                        : null,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: TextField(
                      key: const ValueKey('chat_input_field'),
                      controller: widget.controller,
                      focusNode: widget.focusNode,
                      enabled: !widget.isSending && !_isTranscribing,
                      minLines: 1,
                      maxLines: 6,
                      textInputAction: TextInputAction.newline,
                      decoration: InputDecoration(
                        hintText: _isTranscribing ? 'Распознавание...' : widget.hintText,
                        border: const OutlineInputBorder(),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  if (widget.onStop != null) ...[
                    IconButton(
                      tooltip: widget.stopTooltip,
                      // Стоп активен именно когда идёт операция (isSending);
                      // блокируем только во время распознавания речи.
                      onPressed: _isTranscribing ? null : widget.onStop,
                      style: widget.isStopActive
                          ? IconButton.styleFrom(
                              foregroundColor: theme.colorScheme.error,
                            )
                          : null,
                      icon: const Icon(Icons.stop_circle_outlined),
                    ),
                    const SizedBox(width: 8),
                  ],
                  if (_isTranscribing)
                    const Padding(
                      padding: EdgeInsets.symmetric(horizontal: 12, vertical: 12),
                      child: SizedBox(
                        width: 24,
                        height: 24,
                        child: CircularProgressIndicator(strokeWidth: 2.5),
                      ),
                    )
                  else
                    _ChatInputSendButton(
                      controller: widget.controller,
                      isSending: widget.isSending,
                      tooltip: widget.sendTooltip,
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
