import 'dart:convert';
import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_chat_controller.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'poll_widget.g.dart';

@riverpod
class PollVoted extends _$PollVoted {
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.first_unlock_this_device,
    ),
    mOptions: MacOsOptions(useDataProtectionKeyChain: false),
  );

  @override
  String? build(String messageId) {
    _load(messageId);
    return null;
  }

  Future<void> _load(String messageId) async {
    if (messageId.isEmpty) {
      return;
    }
    final val = await _storage.read(key: 'poll_voted_$messageId');
    state = val;
  }

  Future<void> vote(String option) async {
    if (messageId.isEmpty) {
      state = option;
      return;
    }
    state = option;
    await _storage.write(key: 'poll_voted_$messageId', value: option);
  }
}

/// A premium, glassmorphic Poll Widget that renders from a JSON payload.
class PollWidget extends ConsumerWidget {
  const PollWidget({
    super.key,
    required this.jsonPayload,
    required this.messageId,
    required this.isStreaming,
  });

  final String jsonPayload;
  final String messageId;
  final bool isStreaming;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    // Parse the payload safely
    var question = 'Poll';
    final options = <String>[];

    try {
      final data = jsonDecode(jsonPayload) as Map<String, dynamic>;
      question = data['question'] as String? ?? 'Poll';
      final rawOptions = data['options'];
      if (rawOptions is List) {
        options.addAll(rawOptions.map((e) => e.toString()));
      }
    } catch (_) {
      // Fallback if parsing fails during streaming or malformed JSON
      return Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: scheme.errorContainer.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: scheme.error.withValues(alpha: 0.3)),
        ),
        child: Text(
          'Malformed Poll Payload:\n$jsonPayload',
          style: theme.textTheme.bodyMedium?.copyWith(color: scheme.error),
        ),
      );
    }

    final votedOption = ref.watch(pollVotedProvider(messageId));
    final hasVoted = votedOption != null;

    return Container(
      margin: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: theme.brightness == Brightness.dark
              ? Colors.white.withValues(alpha: 0.08)
              : Colors.black.withValues(alpha: 0.06),
          width: 1.5,
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.05),
            blurRadius: 12,
            spreadRadius: 2,
          ),
        ],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(16),
        child: BackdropFilter(
          filter: ImageFilter.blur(sigmaX: 10, sigmaY: 10),
          child: Container(
            color: theme.brightness == Brightness.dark
                ? Colors.black.withValues(alpha: 0.2)
                : Colors.white.withValues(alpha: 0.4),
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Icon(
                      Icons.poll_outlined,
                      size: 20,
                      color: scheme.primary,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        question,
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: scheme.onSurface,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 16),
                ...options.map((option) {
                  final isSelected = votedOption == option;
                  final isButtonDisabled = isStreaming || hasVoted;

                  return Padding(
                    padding: const EdgeInsets.only(bottom: 8.0),
                    child: _PollOptionButton(
                      optionText: option,
                      isSelected: isSelected,
                      hasVoted: hasVoted,
                      disabled: isButtonDisabled,
                      onPressed: () async {
                        if (isButtonDisabled) {
                          return;
                        }
                        
                        // 1. Record the vote locally
                        await ref.read(pollVotedProvider(messageId).notifier).vote(option);
                        
                        // 2. Send the message back to the assistant chat
                        ref.read(assistantChatControllerProvider.notifier).sendMessage(option);
                      },
                    ),
                  );
                }),
                if (hasVoted)
                  Padding(
                    padding: const EdgeInsets.only(top: 8.0),
                    child: Center(
                      child: Text(
                        'Thank you for your response!',
                        style: theme.textTheme.bodySmall?.copyWith(
                          color: scheme.primary.withValues(alpha: 0.8),
                          fontStyle: FontStyle.italic,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _PollOptionButton extends StatefulWidget {
  const _PollOptionButton({
    required this.optionText,
    required this.isSelected,
    required this.hasVoted,
    required this.disabled,
    required this.onPressed,
  });

  final String optionText;
  final bool isSelected;
  final bool hasVoted;
  final bool disabled;
  final VoidCallback onPressed;

  @override
  State<_PollOptionButton> createState() => _PollOptionButtonState();
}

class _PollOptionButtonState extends State<_PollOptionButton> {
  bool _isHovered = false;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    Color bg;
    BoxBorder border;
    if (widget.isSelected) {
      bg = scheme.primary.withValues(alpha: 0.15);
      border = Border.all(color: scheme.primary, width: 2);
    } else if (widget.hasVoted) {
      bg = theme.brightness == Brightness.dark
          ? Colors.white.withValues(alpha: 0.02)
          : Colors.black.withValues(alpha: 0.01);
      border = Border.all(
        color: theme.brightness == Brightness.dark
            ? Colors.white.withValues(alpha: 0.04)
            : Colors.black.withValues(alpha: 0.03),
        width: 1,
      );
    } else if (_isHovered) {
      bg = scheme.primary.withValues(alpha: 0.05);
      border = Border.all(color: scheme.primary.withValues(alpha: 0.5), width: 1.5);
    } else {
      bg = theme.brightness == Brightness.dark
          ? Colors.white.withValues(alpha: 0.04)
          : Colors.black.withValues(alpha: 0.02);
      border = Border.all(
        color: theme.brightness == Brightness.dark
            ? Colors.white.withValues(alpha: 0.08)
            : Colors.black.withValues(alpha: 0.05),
        width: 1,
      );
    }

    return MouseRegion(
      onEnter: (_) => setState(() => _isHovered = true),
      onExit: (_) => setState(() => _isHovered = false),
      child: GestureDetector(
        onTap: widget.disabled ? null : widget.onPressed,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 150),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: bg,
            borderRadius: BorderRadius.circular(12),
            border: border,
          ),
          child: Row(
            children: [
              Expanded(
                child: Text(
                  widget.optionText,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: widget.isSelected ? FontWeight.bold : FontWeight.normal,
                    color: widget.isSelected
                        ? scheme.primary
                        : widget.hasVoted
                            ? scheme.onSurface.withValues(alpha: 0.5)
                            : scheme.onSurface,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              if (widget.isSelected)
                Icon(
                  Icons.check_circle,
                  color: scheme.primary,
                  size: 20,
                )
              else if (!widget.hasVoted)
                Icon(
                  Icons.radio_button_unchecked,
                  color: _isHovered ? scheme.primary.withValues(alpha: 0.7) : scheme.onSurfaceVariant.withValues(alpha: 0.5),
                  size: 20,
                )
              else
                const SizedBox(width: 20, height: 20),
            ],
          ),
        ),
      ),
    );
  }
}
