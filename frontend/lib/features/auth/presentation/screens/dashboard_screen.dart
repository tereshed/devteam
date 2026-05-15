import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:go_router/go_router.dart';

/// DashboardScreen - главный экран после авторизации
///
/// Отображает информацию о пользователе и предоставляет доступ к функциям приложения.
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'dashboardScreen');
    final authState = ref.watch(authControllerProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.dashboard),
        actions: const [LogoutButton()],
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          child: Container(
            alignment: Alignment.topCenter,
            child: AdaptiveContainer(
              child: authState.when(
                data: (user) {
                  if (user == null) {
                    return Center(child: Text(l10n.userNotAuthorized));
                  }

                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        l10n.welcomeBack,
                        style: Theme.of(context).textTheme.displaySmall,
                        textAlign: TextAlign.center,
                      ),
                      SizedBox(height: Spacing.large(context)),
                      Card(
                        child: Padding(
                          padding: Spacing.cardPadding(context),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                l10n.userInfo,
                                style: Theme.of(context).textTheme.titleLarge,
                              ),
                              SizedBox(height: Spacing.small(context)),
                              _InfoRow(label: l10n.email, value: user.email),
                              SizedBox(height: Spacing.mini(context)),
                              _InfoRow(label: l10n.role, value: user.role),
                              SizedBox(height: Spacing.mini(context)),
                              _InfoRow(
                                label: l10n.emailVerified,
                                value: user.emailVerified ? l10n.yes : l10n.no,
                              ),
                            ],
                          ),
                        ),
                      ),
                      SizedBox(height: Spacing.medium(context)),
                      ElevatedButton.icon(
                        onPressed: () {
                          context.go(AppRoutePaths.settings);
                        },
                        icon: const Icon(Icons.tune),
                        label: Text(l10n.globalSettingsScreenTitle),
                        style: ElevatedButton.styleFrom(
                          padding: Spacing.buttonPadding(context),
                        ),
                      ),
                      SizedBox(height: Spacing.small(context)),
                      ElevatedButton.icon(
                        onPressed: () {
                          context.go('/profile');
                        },
                        icon: const Icon(Icons.person),
                        label: Text(l10n.goToProfile),
                        style: ElevatedButton.styleFrom(
                          padding: Spacing.buttonPadding(context),
                        ),
                      ),
                      if (user.role == 'admin') ...[
                        SizedBox(height: Spacing.medium(context)),
                        _AdminMenuButton(
                          label: l10n.dashboardAdminManagePrompts,
                          icon: Icons.settings_system_daydream,
                          route: '/admin/prompts',
                        ),
                        _AdminMenuButton(
                          label: l10n.dashboardAdminManageWorkflows,
                          icon: Icons.play_circle_outline,
                          route: '/admin/workflows',
                        ),
                        _AdminMenuButton(
                          label: l10n.dashboardAdminViewLlmLogs,
                          icon: Icons.receipt_long,
                          route: '/admin/logs',
                        ),
                        _AdminMenuButton(
                          label: l10n.dashboardAdminAgentsV2,
                          icon: Icons.psychology,
                          route: '/admin/agents-v2',
                        ),
                        _AdminMenuButton(
                          label: l10n.dashboardAdminWorktrees,
                          icon: Icons.account_tree,
                          route: '/admin/worktrees',
                        ),
                      ],
                    ],
                  );
                },
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (error, stackTrace) => Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        l10n.dataLoadError,
                        style: Theme.of(context).textTheme.titleLarge,
                      ),
                      SizedBox(height: Spacing.small(context)),
                      Text(error.toString()),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _AdminMenuButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final String route;

  const _AdminMenuButton({
    required this.label,
    required this.icon,
    required this.route,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: Spacing.small(context)),
      child: ElevatedButton.icon(
        onPressed: () => context.go(route),
        icon: Icon(icon),
        label: Text(label),
        style: ElevatedButton.styleFrom(
          backgroundColor: Theme.of(context).colorScheme.secondaryContainer,
          foregroundColor: Theme.of(context).colorScheme.onSecondaryContainer,
          padding: Spacing.buttonPadding(context),
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;

  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 120,
          child: Text(
            '$label:',
            style: Theme.of(
              context,
            ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
          ),
        ),
        Expanded(
          child: Text(value, style: Theme.of(context).textTheme.bodyMedium),
        ),
      ],
    );
  }
}
