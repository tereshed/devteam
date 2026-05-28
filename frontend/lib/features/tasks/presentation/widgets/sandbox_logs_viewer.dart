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
    this.fillParent = false,
  });

  final String projectId;
  final String taskId;
  final bool fillParent;

  @override
  ConsumerState<SandboxLogsViewer> createState() => _SandboxLogsViewerState();
}

class _SandboxLogsViewerState extends ConsumerState<SandboxLogsViewer> {
  final ScrollController _scrollController = ScrollController();
  final TextEditingController _searchController = TextEditingController();
  final FocusNode _searchFocusNode = FocusNode();
  bool _userScrolledUp = false;
  int _lastLogCount = 0;
  String _searchQuery = '';
  bool _showSearch = false;

  @override
  void initState() {
    super.initState();
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.removeListener(_onScroll);
    _scrollController.dispose();
    _searchController.dispose();
    _searchFocusNode.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (!_scrollController.hasClients) return;
    final pos = _scrollController.position;
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

    if (_userScrolledUp) return;

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

        // Filter logs based on search query
        final filteredLogs = _searchQuery.isEmpty
            ? logs
            : logs
                .where((l) => l.line.toLowerCase().contains(_searchQuery.toLowerCase()))
                .toList();

        _scrollToBottomIfNeeded(filteredLogs.length);

        final fullLogsText = filteredLogs.map((l) {
          final prefix = l.stream == 'stderr' ? '[stderr] ' : '';
          return '$prefix${l.line}';
        }).join('\n');

        final terminalContent = Container(
          color: Colors.black87,
          height: widget.fillParent ? null : 250,
          padding: const EdgeInsets.all(12),
          child: filteredLogs.isEmpty
              ? Center(
                  child: Padding(
                    padding: const EdgeInsets.all(16.0),
                    child: Text(
                      _searchQuery.isEmpty
                          ? l10n.taskDetailSandboxLogsEmpty
                          : 'Ничего не найдено',
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
                    itemCount: filteredLogs.length + (isActive && _searchQuery.isEmpty ? 1 : 0),
                    itemBuilder: (context, index) {
                      if (index == filteredLogs.length) {
                        return const Padding(
                          padding: EdgeInsets.symmetric(vertical: 2),
                          child: Row(
                            children: [
                              _PulsingCursor(),
                            ],
                          ),
                        );
                      }

                      final logEvent = filteredLogs[index];
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
        );

        final cardWidget = Card(
          margin: EdgeInsets.zero,
          elevation: 4,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: BorderSide(color: scheme.outlineVariant),
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Terminal Top Header Bar
              Container(
                color: theme.brightness == Brightness.dark
                    ? Colors.grey.shade900
                    : Colors.grey.shade200,
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
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
                    // Title / Search Input
                    Expanded(
                      child: _showSearch
                          ? SizedBox(
                              height: 32,
                              child: TextField(
                                focusNode: _searchFocusNode,
                                controller: _searchController,
                                style: TextStyle(
                                  fontFamily: 'monospace',
                                  fontSize: 12,
                                  color: theme.brightness == Brightness.dark
                                      ? Colors.white
                                      : Colors.black87,
                                ),
                                decoration: InputDecoration(
                                  hintText: 'Поиск по логам...',
                                  hintStyle: TextStyle(
                                    fontFamily: 'monospace',
                                    fontSize: 11,
                                    color: theme.brightness == Brightness.dark
                                        ? Colors.grey.shade600
                                        : Colors.grey.shade500,
                                  ),
                                  border: InputBorder.none,
                                  isDense: true,
                                  contentPadding: const EdgeInsets.symmetric(vertical: 8),
                                  suffixIcon: IconButton(
                                    padding: EdgeInsets.zero,
                                    constraints: const BoxConstraints(),
                                    icon: const Icon(Icons.close, size: 14, color: Colors.grey),
                                    onPressed: () {
                                      setState(() {
                                        _searchController.clear();
                                        _searchQuery = '';
                                        _showSearch = false;
                                      });
                                    },
                                  ),
                                ),
                                onChanged: (val) {
                                  setState(() {
                                    _searchQuery = val;
                                  });
                                },
                              ),
                            )
                          : Text(
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
                    // Action Buttons
                    IconButton(
                      visualDensity: VisualDensity.compact,
                      icon: Icon(
                        _showSearch ? Icons.search_off : Icons.search,
                        size: 16,
                        color: _showSearch ? scheme.primary : null,
                      ),
                      tooltip: 'Поиск по логам (Ctrl+F)',
                      onPressed: () {
                        setState(() {
                          _showSearch = !_showSearch;
                          if (!_showSearch) {
                            _searchController.clear();
                            _searchQuery = '';
                          } else {
                            WidgetsBinding.instance.addPostFrameCallback((_) {
                              _searchFocusNode.requestFocus();
                            });
                          }
                        });
                      },
                    ),
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
              if (widget.fillParent) Expanded(child: terminalContent) else terminalContent,
            ],
          ),
        );

        return Focus(
          onKeyEvent: (FocusNode node, KeyEvent event) {
            if (event is KeyDownEvent) {
              final isCtrlOrCmd = HardwareKeyboard.instance.isControlPressed ||
                  HardwareKeyboard.instance.isMetaPressed;
              if (event.logicalKey == LogicalKeyboardKey.keyF && isCtrlOrCmd) {
                setState(() {
                  _showSearch = true;
                });
                WidgetsBinding.instance.addPostFrameCallback((_) {
                  _searchFocusNode.requestFocus();
                });
                return KeyEventResult.handled;
              }
              if (event.logicalKey == LogicalKeyboardKey.escape && _showSearch) {
                setState(() {
                  _searchController.clear();
                  _searchQuery = '';
                  _showSearch = false;
                });
                return KeyEventResult.handled;
              }
            }
            return KeyEventResult.ignored;
          },
          child: cardWidget,
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
    );
    final isWidgetTest = WidgetsBinding.instance.runtimeType.toString().contains('Test');
    if (!isWidgetTest) {
      _animationController.repeat(reverse: true);
    }
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
