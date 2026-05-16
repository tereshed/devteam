import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Группа разделов в сайдбаре.
///
/// Используется только для визуального разделения (заголовок группы + дивайдер).
enum AppShellDestinationGroup { home, resources, integrations, admin, settings }

/// Описание одного пункта в [AppShell]'е.
///
/// Не Freezed, потому что [label] вычисляется из [AppLocalizations] runtime'но;
/// держать в freezed было бы лишним для статической константы.
class AppShellDestination {
  final IconData icon;

  /// Иконка-выделение для активного пункта (опционально).
  final IconData? selectedIcon;

  /// Текст пункта меню в текущей локали.
  final String Function(AppLocalizations l10n) label;

  /// Путь GoRouter'а. Совпадение с `GoRouterState.matchedLocation` определяет,
  /// какой пункт «активен».
  final String route;

  /// Только для админов?
  final bool adminOnly;

  /// Группа для визуальной группировки (см. [AppShellDestinationGroup]).
  final AppShellDestinationGroup group;

  const AppShellDestination({
    required this.icon,
    required this.label,
    required this.route,
    required this.group,
    this.selectedIcon,
    this.adminOnly = false,
  });
}

/// Статический список разделов навигации.
///
/// SSOT — берём из dashboard-redesign-plan.md §3 (Информационная архитектура).
/// Любое добавление/удаление маршрута идёт сюда + в `app_router.dart`.
List<AppShellDestination> appShellDestinations() => const [
      AppShellDestination(
        icon: Icons.dashboard_outlined,
        selectedIcon: Icons.dashboard,
        label: _labelDashboard,
        route: '/dashboard',
        group: AppShellDestinationGroup.home,
      ),
      AppShellDestination(
        icon: Icons.folder_outlined,
        selectedIcon: Icons.folder,
        label: _labelProjects,
        route: '/projects',
        group: AppShellDestinationGroup.resources,
      ),
      AppShellDestination(
        icon: Icons.psychology_outlined,
        selectedIcon: Icons.psychology,
        label: _labelAgents,
        route: '/admin/agents-v2',
        group: AppShellDestinationGroup.resources,
        adminOnly: true,
      ),
      AppShellDestination(
        icon: Icons.account_tree_outlined,
        selectedIcon: Icons.account_tree,
        label: _labelWorktrees,
        route: '/admin/worktrees',
        group: AppShellDestinationGroup.resources,
        adminOnly: true,
      ),
      AppShellDestination(
        icon: Icons.power_outlined,
        selectedIcon: Icons.power,
        label: _labelIntegrationsLlm,
        route: '/integrations/llm',
        group: AppShellDestinationGroup.integrations,
      ),
      AppShellDestination(
        icon: Icons.merge_outlined,
        selectedIcon: Icons.merge,
        label: _labelIntegrationsGit,
        route: '/integrations/git',
        group: AppShellDestinationGroup.integrations,
      ),
      AppShellDestination(
        icon: Icons.settings_system_daydream_outlined,
        selectedIcon: Icons.settings_system_daydream,
        label: _labelPrompts,
        route: '/admin/prompts',
        group: AppShellDestinationGroup.admin,
        adminOnly: true,
      ),
      AppShellDestination(
        icon: Icons.play_circle_outline,
        selectedIcon: Icons.play_circle,
        label: _labelWorkflows,
        route: '/admin/workflows',
        group: AppShellDestinationGroup.admin,
        adminOnly: true,
      ),
      AppShellDestination(
        icon: Icons.receipt_long_outlined,
        selectedIcon: Icons.receipt_long,
        label: _labelExecutions,
        route: '/admin/executions',
        group: AppShellDestinationGroup.admin,
        adminOnly: true,
      ),
      AppShellDestination(
        icon: Icons.tune_outlined,
        selectedIcon: Icons.tune,
        label: _labelSettings,
        route: '/settings',
        group: AppShellDestinationGroup.settings,
      ),
      AppShellDestination(
        icon: Icons.person_outline,
        selectedIcon: Icons.person,
        label: _labelProfile,
        route: '/profile',
        group: AppShellDestinationGroup.settings,
      ),
      AppShellDestination(
        icon: Icons.vpn_key_outlined,
        selectedIcon: Icons.vpn_key,
        label: _labelApiKeys,
        route: '/profile/api-keys',
        group: AppShellDestinationGroup.settings,
      ),
    ];

String _labelDashboard(AppLocalizations l) => l.navDashboard;
String _labelProjects(AppLocalizations l) => l.navProjects;
String _labelAgents(AppLocalizations l) => l.navAgents;
String _labelWorktrees(AppLocalizations l) => l.navWorktrees;
String _labelIntegrationsLlm(AppLocalizations l) => l.navIntegrationsLlm;
String _labelIntegrationsGit(AppLocalizations l) => l.navIntegrationsGit;
String _labelPrompts(AppLocalizations l) => l.navPrompts;
String _labelWorkflows(AppLocalizations l) => l.navWorkflows;
String _labelExecutions(AppLocalizations l) => l.navExecutions;
String _labelSettings(AppLocalizations l) => l.navSettings;
String _labelProfile(AppLocalizations l) => l.navProfile;
String _labelApiKeys(AppLocalizations l) => l.navApiKeys;

/// Локализованный заголовок группы.
String appShellGroupLabel(
  AppLocalizations l10n,
  AppShellDestinationGroup group,
) {
  switch (group) {
    case AppShellDestinationGroup.home:
      return l10n.navGroupHome;
    case AppShellDestinationGroup.resources:
      return l10n.navGroupResources;
    case AppShellDestinationGroup.integrations:
      return l10n.navGroupIntegrations;
    case AppShellDestinationGroup.admin:
      return l10n.navGroupAdmin;
    case AppShellDestinationGroup.settings:
      return l10n.navGroupSettings;
  }
}
