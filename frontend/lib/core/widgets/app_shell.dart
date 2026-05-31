import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/app_shell_destinations.dart';
import 'package:frontend/core/widgets/breadcrumb.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_sidebar_controller.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_sidebar.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:go_router/go_router.dart';

/// Общий shell приложения в стиле GCP Cloud Console.
///
/// Включает:
/// * AppBar с breadcrumb + меню пользователя;
/// * NavigationRail (desktop / tablet) или Drawer (mobile) с группированными
///   пунктами меню;
/// * Content slot для дочернего экрана GoRouter'а.
///
/// Используется через `ShellRoute(builder: (ctx, st, child) => AppShell(child: child))`
/// в `core/routing/app_router.dart`.
///
/// Breakpoints (см. `core/utils/responsive.dart`):
/// * `< 600` — Drawer + burger.
/// * `600..1200` — компактный NavigationRail (только иконки + tooltip).
/// * `>= 1200` — расширенный NavigationRail с лейблами.
class AppShell extends ConsumerWidget {
  final Widget child;

  /// Текущий matched location (`GoRouterState.matchedLocation`) — передаётся
  /// из `ShellRoute.builder`, чтобы внутренние виджеты (breadcrumb,
  /// выделение активного пункта rail) не плодили зависимости от внешнего GoRouter API.
  final String location;

  const AppShell({super.key, required this.child, required this.location});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'AppShell');
    final device = Responsive.getDeviceType(context);
    final isMobile = device == DeviceType.mobile;

    final authState = ref.watch(authControllerProvider);
    final isAdmin = authState.maybeWhen(
      data: (user) => user?.role == 'admin',
      orElse: () => false,
    );
    final destinations = appShellDestinations()
        .where((d) => !d.adminOnly || isAdmin)
        .toList(growable: false);

    final selectedIndex = _selectedIndex(destinations, location);

    final sidebarOpen = ref.watch(
      assistantSidebarControllerProvider.select((s) => s.open),
    );

    final appBar = AppBar(
      elevation: 0,
      scrolledUnderElevation: 1,
      title: Row(
        children: [
          Text(
            l10n.appShellBrand,
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(width: 16),
          if (!isMobile)
            Flexible(child: Breadcrumb(location: location)),
        ],
      ),
      actions: [
        // Desktop — toggle inline-колонки через провайдер; tablet/mobile —
        // открываем endDrawer через Scaffold (только тут доступ к нему есть).
        Builder(
          builder: (ctx) => IconButton(
            tooltip: l10n.assistantToggleTooltip,
            onPressed: () {
              if (device == DeviceType.desktop) {
                ref
                    .read(assistantSidebarControllerProvider.notifier)
                    .toggleOpen();
              } else {
                Scaffold.of(ctx).openEndDrawer();
              }
            },
            icon: Icon(
              sidebarOpen
                  ? Icons.assistant
                  : Icons.assistant_outlined,
            ),
          ),
        ),
        const Padding(
          padding: EdgeInsets.only(right: 8),
          child: LogoutButton(),
        ),
      ],
    );

    // endDrawer для assistant sidebar на mobile/tablet (план §2 frontend).
    // Desktop держит панель встроенной колонкой в Row ниже.
    final endDrawer = device != DeviceType.desktop
        ? const Drawer(
            child: SafeArea(child: SizedBox(width: 360, child: AssistantSidebar())),
          )
        : null;

    if (isMobile) {
      return Scaffold(
        appBar: appBar,
        drawer: Drawer(
          child: SafeArea(
            child: _DestinationsList(
              destinations: destinations,
              selectedIndex: selectedIndex,
              location: location,
              expanded: true,
              onTap: (route) {
                Navigator.of(context).pop();
                context.go(route);
              },
            ),
          ),
        ),
        endDrawer: endDrawer,
        body: SafeArea(
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Breadcrumb(location: location),
                ),
              ),
              Expanded(child: child),
            ],
          ),
        ),
      );
    }

    // Desktop: меню можно свернуть до иконок (tablet всегда компактный).
    final railExpanded = ref.watch(navRailExpandedProvider);
    final extended = device == DeviceType.desktop && railExpanded;
    final railWidth = extended ? 240.0 : 80.0;
    const assistantWidth = 360.0;
    final showInlineAssistant = extended && sidebarOpen;

    return Scaffold(
      appBar: appBar,
      endDrawer: extended ? null : endDrawer,
      body: SafeArea(
        child: Row(
          children: [
            SizedBox(
              width: railWidth,
              child: Material(
                color: Theme.of(context).colorScheme.surface,
                elevation: 0,
                child: _DestinationsList(
                  destinations: destinations,
                  selectedIndex: selectedIndex,
                  location: location,
                  expanded: extended,
                  onToggleCollapse: device == DeviceType.desktop
                      ? () =>
                          ref.read(navRailExpandedProvider.notifier).toggle()
                      : null,
                  onTap: (route) => context.go(route),
                ),
              ),
            ),
            const VerticalDivider(width: 1, thickness: 1),
            Expanded(child: child),
            // Desktop: правая колонка-ассистент, collapsible. Сворачиваем
            // через AnimatedSize, чтобы layout перестраивался плавно (§8).
            AnimatedSize(
              duration: const Duration(milliseconds: 180),
              alignment: Alignment.centerRight,
              child: showInlineAssistant
                  ? const Row(
                      children: [
                        VerticalDivider(width: 1, thickness: 1),
                        SizedBox(
                          width: assistantWidth,
                          child: AssistantSidebar(),
                        ),
                      ],
                    )
                  : const SizedBox(width: 0, height: 0),
            ),
          ],
        ),
      ),
    );
  }

  static int _selectedIndex(
    List<AppShellDestination> destinations,
    String location,
  ) {
    // Берём самый длинный префикс, чтобы вложенные маршруты (e.g.
    // `/projects/abc`) выделяли свой корневой пункт (`/projects`).
    var bestIdx = -1;
    var bestLen = 0;
    for (var i = 0; i < destinations.length; i++) {
      final route = destinations[i].route;
      if (location == route || location.startsWith('$route/')) {
        if (route.length > bestLen) {
          bestLen = route.length;
          bestIdx = i;
        }
      }
    }
    return bestIdx;
  }
}

class _DestinationsList extends StatelessWidget {
  final List<AppShellDestination> destinations;
  final int selectedIndex;
  final String location;
  final bool expanded;
  final ValueChanged<String> onTap;

  /// Тогл сворачивания меню (только desktop). null — кнопка не показывается.
  final VoidCallback? onToggleCollapse;

  const _DestinationsList({
    required this.destinations,
    required this.selectedIndex,
    required this.location,
    required this.expanded,
    required this.onTap,
    this.onToggleCollapse,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AppShell._DestinationsList',
    );
    final theme = Theme.of(context);

    final items = <Widget>[];

    // Кнопка сворачивания меню до иконок (desktop).
    if (onToggleCollapse != null) {
      items.add(
        Padding(
          padding: EdgeInsets.symmetric(horizontal: expanded ? 12 : 8),
          child: Align(
            alignment: expanded ? Alignment.centerRight : Alignment.center,
            child: IconButton(
              onPressed: onToggleCollapse,
              icon: Icon(expanded ? Icons.menu_open : Icons.menu),
              iconSize: 20,
              tooltip:
                  expanded ? l10n.appShellNavCollapse : l10n.appShellNavExpand,
            ),
          ),
        ),
      );
    }

    AppShellDestinationGroup? lastGroup;
    for (var i = 0; i < destinations.length; i++) {
      final d = destinations[i];
      if (lastGroup != null && lastGroup != d.group) {
        items.add(const Divider(height: 16, thickness: 1));
      }
      if (expanded && lastGroup != d.group) {
        items.add(
          Padding(
            padding: const EdgeInsets.fromLTRB(20, 12, 16, 6),
            child: Text(
              appShellGroupLabel(l10n, d.group),
              style: theme.textTheme.labelSmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
                letterSpacing: 0.8,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
        );
      }
      lastGroup = d.group;
      items.add(
        _DestinationTile(
          destination: d,
          selected: i == selectedIndex,
          expanded: expanded,
          onTap: () => onTap(d.route),
        ),
      );
    }

    return ListView(
      padding: const EdgeInsets.symmetric(vertical: 8),
      children: items,
    );
  }
}

class _DestinationTile extends StatelessWidget {
  final AppShellDestination destination;
  final bool selected;
  final bool expanded;
  final VoidCallback onTap;

  const _DestinationTile({
    required this.destination,
    required this.selected,
    required this.expanded,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'AppShell._DestinationTile',
    );
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;
    final iconData = selected
        ? (destination.selectedIcon ?? destination.icon)
        : destination.icon;
    final iconColor =
        selected ? scheme.primary : scheme.onSurfaceVariant;
    final label = destination.label(l10n);

    final content = Container(
      margin: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: selected ? scheme.secondaryContainer : Colors.transparent,
        borderRadius: BorderRadius.circular(12),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: expanded
              ? const EdgeInsets.symmetric(horizontal: 12, vertical: 10)
              : const EdgeInsets.symmetric(horizontal: 8, vertical: 12),
          child: expanded
              ? Row(
                  children: [
                    Icon(iconData, size: 22, color: iconColor),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Text(
                        label,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight:
                              selected ? FontWeight.w600 : FontWeight.w400,
                          color: selected
                              ? scheme.onSecondaryContainer
                              : scheme.onSurface,
                        ),
                      ),
                    ),
                  ],
                )
              : Center(
                  child: Icon(iconData, size: 22, color: iconColor),
                ),
        ),
      ),
    );

    if (expanded) {
      return content;
    }
    return Tooltip(message: label, child: content);
  }
}

/// Раскрыто ли основное меню (NavigationRail) на desktop. По умолчанию — да;
/// пользователь может свернуть его до иконок. Состояние живёт в корне
/// ProviderScope, поэтому сохраняется при навигации между маршрутами.
class NavRailExpandedNotifier extends Notifier<bool> {
  @override
  bool build() => true;

  void toggle() => state = !state;
}

final navRailExpandedProvider =
    NotifierProvider<NavRailExpandedNotifier, bool>(
  NavRailExpandedNotifier.new,
);
