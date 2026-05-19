import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
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
    final statusAsync = ref.watch(assistantStatusProvider);
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
          Expanded(
            child: statusAsync.when(
              data: (status) {
                if (!status.isConfigured) {
                  return _AssistantLockScreen(requiredProvider: status.requiredProvider);
                }
                return Column(
                  children: [
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
                );
              },
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, st) => Center(child: Text(l10n.assistantStatusError(err.toString()))),
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
          // ValueKey на label — единственное место в ButtonSegment, куда
          // можно подвесить ключ. Тесты ищут вкладки по этим ключам, чтобы
          // не зависеть от локализации (см. docs/rules/frontend.md i18n).
          label: Text(chatLabel, key: const ValueKey('assistant_tab_chat')),
          icon: const Icon(Icons.chat_bubble_outline, size: 16),
        ),
        ButtonSegment(
          value: AssistantSidebarTab.tasks,
          label: Text(tasksLabel, key: const ValueKey('assistant_tab_tasks')),
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

class _AssistantLockScreen extends StatelessWidget {
  const _AssistantLockScreen({required this.requiredProvider});
  final String requiredProvider;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AssistantSidebar');
    final theme = Theme.of(context);
    final isAdminSetup = requiredProvider == 'admin_setup_required';
    
    return Padding(
      padding: const EdgeInsets.all(24.0),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.lock_outline, size: 48, color: theme.colorScheme.onSurfaceVariant),
          const SizedBox(height: 16),
          Text(
            isAdminSetup ? l10n.assistantStatusAdminSetup : l10n.assistantLockScreenMessage,
            textAlign: TextAlign.center,
            style: theme.textTheme.bodyLarge,
          ),
          if (!isAdminSetup) ...[
            const SizedBox(height: 24),
            FilledButton.icon(
              onPressed: () {
                context.goNamed(AppRouteNames.globalSettings);
              },
              icon: const Icon(Icons.settings),
              label: Text(l10n.assistantLockScreenButton),
            ),
          ],
        ],
      ),
    );
  }
}
