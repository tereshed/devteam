import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_sidebar_controller.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_chat_panel.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tasks_panel.dart';

/// Главный контейнер правой панели ассистента (Sprint 21 §1 frontend).
///
/// Header (заголовок + TabBar) + body (вкладка по выбору).
/// AppShell оборачивает это в нужный layout (фиксированная колонка / Drawer
/// endDrawer) в зависимости от breakpoint.
class AssistantSidebar extends ConsumerWidget {
  const AssistantSidebar({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AssistantSidebar');
    final sidebar = ref.watch(assistantSidebarControllerProvider);
    final notifier = ref.read(assistantSidebarControllerProvider.notifier);
    final theme = Theme.of(context);

    return Material(
      color: theme.colorScheme.surface,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 4, 0),
            child: Row(
              children: [
                Icon(Icons.assistant_outlined,
                    color: theme.colorScheme.primary, size: 20),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    l10n.assistantSidebarTitle,
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                IconButton(
                  tooltip: l10n.assistantToggleTooltip,
                  onPressed: notifier.toggleOpen,
                  icon: const Icon(Icons.chevron_right),
                ),
              ],
            ),
          ),
          _AssistantTabBar(
            current: sidebar.tab,
            onChanged: notifier.setTab,
            chatLabel: l10n.assistantTabChat,
            tasksLabel: l10n.assistantTabTasks,
          ),
          const Divider(height: 1),
          Expanded(
            child: AnimatedSwitcher(
              duration: const Duration(milliseconds: 160),
              child: switch (sidebar.tab) {
                AssistantSidebarTab.chat => const AssistantChatPanel(
                    key: ValueKey('assistant_chat_panel'),
                  ),
                AssistantSidebarTab.tasks => const AssistantTasksPanel(
                    key: ValueKey('assistant_tasks_panel'),
                  ),
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _AssistantTabBar extends StatelessWidget {
  const _AssistantTabBar({
    required this.current,
    required this.onChanged,
    required this.chatLabel,
    required this.tasksLabel,
  });

  final AssistantSidebarTab current;
  final ValueChanged<AssistantSidebarTab> onChanged;
  final String chatLabel;
  final String tasksLabel;

  @override
  Widget build(BuildContext context) {
    return SegmentedButton<AssistantSidebarTab>(
      segments: [
        ButtonSegment(
          value: AssistantSidebarTab.chat,
          label: Text(chatLabel),
          icon: const Icon(Icons.chat_bubble_outline, size: 16),
        ),
        ButtonSegment(
          value: AssistantSidebarTab.tasks,
          label: Text(tasksLabel),
          icon: const Icon(Icons.task_alt_outlined, size: 16),
        ),
      ],
      selected: <AssistantSidebarTab>{current},
      onSelectionChanged: (set) {
        if (set.isEmpty) return;
        onChanged(set.first);
      },
      showSelectedIcon: false,
    );
  }
}
