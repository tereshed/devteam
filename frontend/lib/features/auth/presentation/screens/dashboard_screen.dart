import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// DashboardScreen - главный экран после авторизации
///
/// Отображает информацию о пользователе и предоставляет доступ к функциям приложения.
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
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
                        ElevatedButton.icon(
                          onPressed: () => context.go('/admin/prompts'),
                          icon: const Icon(Icons.settings_system_daydream),
                          label: const Text('Manage Prompts (Admin)'),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Theme.of(
                              context,
                            ).colorScheme.secondaryContainer,
                            foregroundColor: Theme.of(
                              context,
                            ).colorScheme.onSecondaryContainer,
                            padding: Spacing.buttonPadding(context),
                          ),
                        ),
                        SizedBox(height: Spacing.small(context)),
                        ElevatedButton.icon(
                          onPressed: () => context.go('/admin/workflows'),
                          icon: const Icon(Icons.play_circle_outline),
                          label: const Text('Manage Workflows (Admin)'),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Theme.of(
                              context,
                            ).colorScheme.secondaryContainer,
                            foregroundColor: Theme.of(
                              context,
                            ).colorScheme.onSecondaryContainer,
                            padding: Spacing.buttonPadding(context),
                          ),
                        ),
                        SizedBox(height: Spacing.small(context)),
                        ElevatedButton.icon(
                          onPressed: () => context.go('/admin/logs'),
                          icon: const Icon(Icons.receipt_long),
                          label: const Text('View LLM Logs (Admin)'),
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Theme.of(
                              context,
                            ).colorScheme.secondaryContainer,
                            foregroundColor: Theme.of(
                              context,
                            ).colorScheme.onSecondaryContainer,
                            padding: Spacing.buttonPadding(context),
                          ),
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
