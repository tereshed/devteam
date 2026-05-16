import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Узел в цепочке breadcrumb'а.
///
/// `route` — путь, на который тапается элемент (если он не текущий).
/// `label` — локализованный человеко-читаемый кусок.
class BreadcrumbNode {
  final String label;

  /// `null` для последнего элемента (он не кликабельный).
  final String? route;

  const BreadcrumbNode({required this.label, this.route});
}

/// Виджет breadcrumb для AppBar'а.
///
/// Принимает текущий [GoRouterState.matchedLocation] и собирает цепочку через
/// статический маппинг сегментов. Идея — простой стейтлесс справочник, без БД-резолва.
class Breadcrumb extends StatelessWidget {
  /// Текущий matched location (e.g. `/integrations/llm`).
  final String location;

  const Breadcrumb({super.key, required this.location});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'Breadcrumb');
    final nodes = buildBreadcrumbNodes(location, l10n);
    if (nodes.isEmpty) {
      return const SizedBox.shrink();
    }

    final theme = Theme.of(context);
    final muted = theme.colorScheme.onSurfaceVariant;

    final children = <Widget>[];
    for (var i = 0; i < nodes.length; i++) {
      final node = nodes[i];
      final isLast = i == nodes.length - 1;
      final style = theme.textTheme.bodyMedium?.copyWith(
        color: isLast ? theme.colorScheme.onSurface : muted,
        fontWeight: isLast ? FontWeight.w600 : FontWeight.w400,
      );
      Widget child = Text(node.label, style: style);
      if (!isLast && node.route != null) {
        child = InkWell(
          borderRadius: BorderRadius.circular(4),
          onTap: () => context.go(node.route!),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
            child: Text(node.label, style: style),
          ),
        );
      } else {
        child = Padding(
          padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
          child: child,
        );
      }
      children.add(child);
      if (!isLast) {
        children.add(Icon(Icons.chevron_right, size: 16, color: muted));
      }
    }

    // Горизонтальный скролл — защита от RenderFlex overflow, когда rail
    // открыт, а сам путь длинный (e.g. `/projects/<uuid>/tasks/<uuid>`).
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: children,
      ),
    );
  }
}

/// Собирает цепочку из [location].
///
/// Вынесено отдельно, чтобы тестировать без `WidgetTester`.
List<BreadcrumbNode> buildBreadcrumbNodes(
  String location,
  AppLocalizations l10n,
) {
  if (location.isEmpty || location == '/') {
    return const [];
  }

  final segments =
      location.split('/').where((s) => s.isNotEmpty).toList(growable: false);
  if (segments.isEmpty) {
    return const [];
  }

  final nodes = <BreadcrumbNode>[BreadcrumbNode(label: l10n.navBreadcrumbHome)];

  var accumulated = '';
  for (var i = 0; i < segments.length; i++) {
    final seg = segments[i];
    accumulated = '$accumulated/$seg';
    final isLast = i == segments.length - 1;
    final label = _labelForSegment(seg, l10n, segments, i);
    nodes.add(
      BreadcrumbNode(
        label: label,
        route: isLast ? null : accumulated,
      ),
    );
  }

  return nodes;
}

String _labelForSegment(
  String segment,
  AppLocalizations l10n,
  List<String> all,
  int index,
) {
  switch (segment) {
    case 'dashboard':
      return l10n.navDashboard;
    case 'projects':
      return l10n.navProjects;
    case 'profile':
      return l10n.navProfile;
    case 'api-keys':
      return l10n.navApiKeys;
    case 'settings':
      return l10n.navSettings;
    case 'integrations':
      return l10n.navGroupIntegrations;
    case 'llm':
      return l10n.navIntegrationsLlm;
    case 'git':
      return l10n.navIntegrationsGit;
    case 'admin':
      return l10n.navGroupAdmin;
    case 'agents-v2':
      return l10n.navAgents;
    case 'worktrees':
      return l10n.navWorktrees;
    case 'prompts':
      return l10n.navPrompts;
    case 'workflows':
      return l10n.navWorkflows;
    case 'executions':
      return l10n.navExecutions;
    case 'new':
      return l10n.navBreadcrumbNew;
    default:
      // Для UUID-сегментов (id записи) — обрезаем до 8 символов.
      if (_looksLikeUuid(segment)) {
        return segment.substring(0, 8);
      }
      return segment;
  }
}

bool _looksLikeUuid(String s) {
  if (s.length < 32) {
    return false;
  }
  final r = RegExp(r'^[0-9a-fA-F-]+$');
  return r.hasMatch(s);
}
