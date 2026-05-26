import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';

class SandboxLogsViewer extends ConsumerStatefulWidget {
  const SandboxLogsViewer({
    super.key,
    required this.projectId,
    required this.taskId,
  });

  final String projectId;
  final String taskId;

  @override
  ConsumerState<SandboxLogsViewer> createState() => _SandboxLogsViewerState();
}

class _SandboxLogsViewerState extends ConsumerState<SandboxLogsViewer> {
  final ScrollController _scrollController = ScrollController();
  bool _userScrolledUp = false;
  int _lastLogCount = 0;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final pos = _scrollController.position;
    // If user is more than 30px away from the bottom, assume they scrolled up manually
    final fromBottom = pos.maxScrollExtent - pos.pixels;
    if (fromBottom > 30) {
      if (!_userScrolledUp) {
        setState(() {
          _userScrolledUp = true;
        });
      }
    } else {
      if (_userScrolledUp) {
        setState(() {
          _userScrolledUp = false;
        });
      }
    }
  }

  void _scrollToBottomIfNeeded(int currentCount) {
    if (currentCount == _lastLogCount) return;
    _lastLogCount = currentCount;

    if (_userScrolledUp) return; // Keep user's scroll position if they scrolled up

    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scrollController.hasClients) return;
      _scrollController.jumpTo(_scrollController.position.maxScrollExtent);
    });
  }

  Future<void> _copyLogsToClipboard(BuildContext context, String logsText, AppLocalizations l10n) async {
    await Clipboard.setData(ClipboardData(text: logsText));
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(l10n.taskDetailSandboxLogsCopied),
        duration: const Duration(seconds: 2),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    final provider = taskDetailControllerProvider(
      projectId: widget.projectId,
      taskId: widget.taskId,
    );
    final asyncDetail = ref.watch(provider);

    return asyncDetail.when(
      loading: () => const SizedBox.shrink(),
      error: (_, __) => const SizedBox.shrink(),
      data: (state) {
        final logs = state.sandboxLogs;
        final task = state.task;
        final isActive = task?.status == 'active';

        // Auto scroll on new logs
        _scrollToBottomIfNeeded(logs.length);

        // Concatenate log text for clipboard copy
        final fullLogsText = logs.map((l) {
          final prefix = l.stream == 'stderr' ? '[stderr] ' : '';
          return '$prefix${l.line}';
        }).join('\n');

        return Card(
          margin: const EdgeInsets.symmetric(vertical: 8),
          elevation: 4,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: BorderSide(color: scheme.outlineVariant),
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Terminal Top Header Bar (macOS Window Style)
              Container(
                color: theme.brightness == Brightness.dark
                    ? Colors.grey.shade900
                    : Colors.grey.shade200,
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                child: Row(
                  children: [
                    // macOS dots
                    Row(
                      children: [
                        _buildDot(Colors.redAccent.shade200),
                        const SizedBox(width: 6),
                        _buildDot(Colors.amber.shade300),
                        const SizedBox(width: 6),
                        _buildDot(Colors.greenAccent.shade400),
                      ],
                    ),
                    const SizedBox(width: 24),
                    // Centered Terminal Title
                    Expanded(
                      child: Text(
                        l10n.taskDetailSectionSandboxLogs,
                        style: theme.textTheme.labelMedium?.copyWith(
                          fontFamily: 'monospace',
                          fontWeight: FontWeight.bold,
                          color: theme.brightness == Brightness.dark
                              ? Colors.grey.shade400
                              : Colors.grey.shade700,
                        ),
                      ),
                    ),
                    // Action Buttons: Copy & Clear
                    if (logs.isNotEmpty) ...[
                      IconButton(
                        visualDensity: VisualDensity.compact,
                        icon: const Icon(Icons.copy, size: 16),
                        tooltip: l10n.taskDetailSandboxLogsCopy,
                        onPressed: () => _copyLogsToClipboard(context, fullLogsText, l10n),
                      ),
                      const SizedBox(width: 4),
                      IconButton(
                        visualDensity: VisualDensity.compact,
                        icon: const Icon(Icons.delete_outline, size: 16),
                        tooltip: l10n.taskDetailSandboxLogsClear,
                        onPressed: () => ref.read(provider.notifier).clearSandboxLogs(),
                      ),
                    ],
                  ],
                ),
              ),
              // Terminal Body
              Container(
                color: Colors.black87,
                height: 250,
                padding: const EdgeInsets.all(12),
                child: logs.isEmpty
                    ? Center(
                        child: Padding(
                          padding: const EdgeInsets.all(16.0),
                          child: Text(
                            l10n.taskDetailSandboxLogsEmpty,
                            textAlign: TextAlign.center,
                            style: const TextStyle(
                              fontFamily: 'monospace',
                              fontSize: 12,
                              color: Colors.grey,
                            ),
                          ),
                        ),
                      )
                    : Scrollbar(
                        controller: _scrollController,
                        thumbVisibility: true,
                        child: ListView.builder(
                          controller: _scrollController,
                          padding: const EdgeInsets.only(right: 8),
                          itemCount: logs.length + (isActive ? 1 : 0),
                          itemBuilder: (context, index) {
                            if (index == logs.length) {
                              return const Padding(
                                padding: EdgeInsets.symmetric(vertical: 2),
                                child: Row(
                                  children: [
                                    _PulsingCursor(),
                                  ],
                                ),
                              );
                            }

                            final logEvent = logs[index];
                            final isStderr = logEvent.stream == 'stderr';

                            return Padding(
                              padding: const EdgeInsets.symmetric(vertical: 1),
                              child: RichText(
                                text: TextSpan(
                                  children: [
                                    TextSpan(
                                      text: isStderr ? '[stderr] ' : '',
                                      style: TextStyle(
                                        fontFamily: 'monospace',
                                        fontSize: 12,
                                        fontWeight: FontWeight.bold,
                                        color: isStderr
                                            ? Colors.redAccent.shade200
                                            : Colors.cyanAccent.shade200,
                                      ),
                                    ),
                                    TextSpan(
                                      text: logEvent.line,
                                      style: const TextStyle(
                                        fontFamily: 'monospace',
                                        fontSize: 12,
                                        color: Colors.white,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            );
                          },
                        ),
                      ),
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildDot(Color color) {
    return Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(
        color: color,
        shape: BoxShape.circle,
      ),
    );
  }
}

class _PulsingCursor extends StatefulWidget {
  const _PulsingCursor();

  @override
  State<_PulsingCursor> createState() => _PulsingCursorState();
}

class _PulsingCursorState extends State<_PulsingCursor>
    with SingleTickerProviderStateMixin {
  late final AnimationController _animationController;

  @override
  void initState() {
    super.initState();
    _animationController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 600),
    )..repeat(reverse: true);
  }

  @override
  void dispose() {
    _animationController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return FadeTransition(
      opacity: _animationController,
      child: const Text(
        '█',
        style: TextStyle(
          color: Colors.greenAccent,
          fontSize: 12,
        ),
      ),
    );
  }
}
