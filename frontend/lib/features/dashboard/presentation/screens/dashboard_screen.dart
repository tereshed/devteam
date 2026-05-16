import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/dashboard/presentation/providers/dashboard_summary_provider.dart';
import 'package:frontend/features/dashboard/presentation/widgets/stat_card.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Hub-экран `/dashboard` — точка входа после логина (см. dashboard-redesign-plan.md §4.2).
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'DashboardScreen');
    final auth = ref.watch(authControllerProvider);
    final summary = ref.watch(dashboardSummaryProvider);
    final recentTasks = ref.watch(dashboardRecentTasksProvider);

    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            auth.maybeWhen(
              data: (user) => user != null
                  ? l10n.dashboardWelcomeUser(user.email)
                  : l10n.dashboardWelcomeAnon,
              orElse: () => l10n.dashboardWelcomeAnon,
            ),
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(height: 4),
          Text(
            l10n.dashboardHubSubtitle,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 24),
          _StatGrid(summary: summary, l10n: l10n),
          const SizedBox(height: 32),
          Text(
            l10n.dashboardRecentTasksTitle,
            style: Theme.of(context).textTheme.titleLarge?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
          const SizedBox(height: 12),
          _RecentTasksBlock(tasks: recentTasks),
        ],
      ),
    );
  }
}

class _StatGrid extends StatelessWidget {
  final AsyncValue<DashboardSummary> summary;
  final AppLocalizations l10n;

  const _StatGrid({required this.summary, required this.l10n});

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        const minCardWidth = 260.0;
        final cols = (constraints.maxWidth / minCardWidth)
            .floor()
            .clamp(1, 4);
        const spacing = 16.0;
        final cardWidth =
            (constraints.maxWidth - spacing * (cols - 1)) / cols;

        // Держим «последние удачные данные», если они есть, иначе нули — но
        // нули показываем ТОЛЬКО в loading-стейте (через '…'). На ошибке без
        // ранее загруженных данных рендерим явный прочерк, см. fmt() ниже.
        const empty = DashboardSummary(
          projectsActive: 0,
          projectsTotal: 0,
          agentsTotal: 0,
          llmConnected: 0,
          gitConnected: 0,
        );
        final data = summary.hasValue ? summary.value! : empty;
        final isLoading = summary.isLoading && !summary.hasValue;
        final hasError = summary.hasError && !summary.hasValue;

        // Универсальный форматтер: явный прочерк на ошибке вместо «0».
        String fmt(String Function() onData) =>
            hasError ? '—' : (isLoading ? '…' : onData());
        String? fmtNullable(String Function() onData) =>
            hasError || isLoading ? null : onData();

        Widget cell(Widget child) =>
            SizedBox(width: cardWidth, child: child);

        return Wrap(
          spacing: spacing,
          runSpacing: spacing,
          children: [
            cell(StatCard(
              icon: Icons.folder,
              title: l10n.navProjects,
              primaryValue:
                  fmt(() => l10n.dashboardStatProjectsActive(data.projectsActive)),
              secondaryValue: fmtNullable(
                () => l10n.dashboardStatProjectsTotal(data.projectsTotal),
              ),
              ctaLabel: l10n.dashboardStatManageCta,
              onTap: () => context.go('/projects'),
            )),
            cell(StatCard(
              icon: Icons.psychology,
              title: l10n.navAgents,
              primaryValue:
                  fmt(() => l10n.dashboardStatAgentsTotal(data.agentsTotal)),
              ctaLabel: l10n.dashboardStatManageCta,
              onTap: () => context.go('/admin/agents-v2'),
            )),
            cell(StatCard(
              icon: Icons.power,
              title: l10n.navIntegrationsLlm,
              primaryValue:
                  fmt(() => l10n.dashboardStatLlmConnected(data.llmConnected)),
              secondaryValue: l10n.dashboardStatComingSoon,
              ctaLabel: l10n.dashboardStatManageCta,
              onTap: () => context.go('/integrations/llm'),
            )),
            cell(StatCard(
              icon: Icons.merge,
              title: l10n.navIntegrationsGit,
              primaryValue:
                  fmt(() => l10n.dashboardStatGitConnected(data.gitConnected)),
              secondaryValue: l10n.dashboardStatComingSoon,
              ctaLabel: l10n.dashboardStatManageCta,
              onTap: () => context.go('/integrations/git'),
            )),
          ],
        );
      },
    );
  }
}

class _RecentTasksBlock extends StatelessWidget {
  final AsyncValue<List<TaskListItemModel>> tasks;

  const _RecentTasksBlock({required this.tasks});

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: '_RecentTasksBlock');
    final theme = Theme.of(context);

    return Card(
      clipBehavior: Clip.antiAlias,
      child: tasks.when(
        loading: () => const Padding(
          padding: EdgeInsets.all(24),
          child: Center(child: CircularProgressIndicator()),
        ),
        error: (e, _) => Padding(
          padding: const EdgeInsets.all(24),
          child: Row(
            children: [
              Icon(Icons.error_outline, color: theme.colorScheme.error),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  l10n.dashboardRecentTasksError,
                  style: theme.textTheme.bodyMedium,
                ),
              ),
            ],
          ),
        ),
        data: (items) {
          if (items.isEmpty) {
            return Padding(
              padding: const EdgeInsets.all(24),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    l10n.dashboardRecentTasksEmptyTitle,
                    style: theme.textTheme.titleSmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    l10n.dashboardRecentTasksEmptySubtitle,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            );
          }
          return Column(
            children: [
              for (final t in items.take(5)) ...[
                ListTile(
                  leading: const Icon(Icons.task_alt),
                  title: Text(
                    t.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  subtitle: Text(
                    t.status,
                    style: theme.textTheme.bodySmall,
                  ),
                  trailing: Text(
                    _formatDate(t.createdAt),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
                const Divider(height: 1),
              ],
            ],
          );
        },
      ),
    );
  }

  static String _formatDate(DateTime dt) {
    final l = dt.toLocal();
    String two(int v) => v.toString().padLeft(2, '0');
    return '${l.year}-${two(l.month)}-${two(l.day)} ${two(l.hour)}:${two(l.minute)}';
  }
}
