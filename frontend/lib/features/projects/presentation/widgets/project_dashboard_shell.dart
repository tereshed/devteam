import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Навигация по разделам проекта (rail ≥600 dp / bar \<600 dp) и область ветки shell.
class ProjectDashboardShell extends StatelessWidget {
  const ProjectDashboardShell({
    super.key,
    required this.navigationShell,
    this.overlay,
  });

  final StatefulNavigationShell navigationShell;
  final Widget? overlay;

  static const double _kBreakpointWidthDp = 600;

  @override
  Widget build(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final useRail = width >= _kBreakpointWidthDp;
    final l10n = AppLocalizations.of(context)!;
    final navInteractive = overlay == null;

    final destinations =
        <
          ({
            IconData icon,
            IconData selectedIcon,
            String label,
          })
        >[
          (
            icon: Icons.chat_outlined,
            selectedIcon: Icons.chat,
            label: l10n.projectDashboardChat,
          ),
          (
            icon: Icons.checklist_outlined,
            selectedIcon: Icons.checklist,
            label: l10n.projectDashboardTasks,
          ),
          (
            icon: Icons.groups_outlined,
            selectedIcon: Icons.groups,
            label: l10n.projectDashboardTeam,
          ),
          (
            icon: Icons.settings_outlined,
            selectedIcon: Icons.settings,
            label: l10n.projectDashboardSettings,
          ),
        ];

    final branchStack = Stack(
      fit: StackFit.expand,
      children: [navigationShell, if (overlay != null) overlay!],
    );

    if (useRail) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          IgnorePointer(
            ignoring: !navInteractive,
            child: NavigationRail(
              selectedIndex: navigationShell.currentIndex,
              onDestinationSelected: navigationShell.goBranch,
              labelType: NavigationRailLabelType.all,
              destinations: [
                for (final d in destinations)
                  NavigationRailDestination(
                    icon: Icon(d.icon),
                    selectedIcon: Icon(d.selectedIcon),
                    label: Text(d.label),
                  ),
              ],
            ),
          ),
          const VerticalDivider(width: 1, thickness: 1),
          Expanded(child: branchStack),
        ],
      );
    }

    return Column(
      children: [
        Expanded(child: branchStack),
        IgnorePointer(
          ignoring: !navInteractive,
          child: NavigationBar(
            selectedIndex: navigationShell.currentIndex,
            onDestinationSelected: navigationShell.goBranch,
            destinations: [
              for (final d in destinations)
                NavigationDestination(
                  icon: Icon(d.icon),
                  selectedIcon: Icon(d.selectedIcon),
                  label: d.label,
                ),
            ],
          ),
        ),
      ],
    );
  }
}
