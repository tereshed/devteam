import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/auth/presentation/widgets/logout_button.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// ProfileScreen - экран профиля пользователя
///
/// Отображает информацию о пользователе и предоставляет возможность выхода.
class ProfileScreen extends ConsumerWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.profile),
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
                      SizedBox(height: Spacing.large(context)),
                      // Аватар пользователя
                      Center(
                        child: CircleAvatar(
                          radius: Spacing.avatarRadius(context),
                          backgroundColor: Theme.of(
                            context,
                          ).colorScheme.primaryContainer,
                          child: Text(
                            user.email[0].toUpperCase(),
                            style: Theme.of(context).textTheme.displayMedium
                                ?.copyWith(
                                  color: Theme.of(
                                    context,
                                  ).colorScheme.onPrimaryContainer,
                                ),
                          ),
                        ),
                      ),
                      SizedBox(height: Spacing.medium(context)),
                      Text(
                        user.email,
                        style: Theme.of(context).textTheme.headlineMedium,
                        textAlign: TextAlign.center,
                      ),
                      SizedBox(height: Spacing.xLarge(context)),
                      // Информация о пользователе
                      Card(
                        child: Padding(
                          padding: Spacing.cardPadding(context),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                l10n.information,
                                style: Theme.of(context).textTheme.titleLarge,
                              ),
                              SizedBox(height: Spacing.small(context)),
                              _InfoRow(
                                context: context,
                                icon: Icons.email,
                                label: l10n.email,
                                value: user.email,
                              ),
                              const Divider(),
                              _InfoRow(
                                context: context,
                                icon: Icons.badge,
                                label: l10n.role,
                                value: user.role,
                              ),
                              const Divider(),
                              _InfoRow(
                                context: context,
                                icon: Icons.verified,
                                label: l10n.emailVerified,
                                value: user.emailVerified ? l10n.yes : l10n.no,
                                valueColor: user.emailVerified
                                    ? Colors.green
                                    : Colors.orange,
                              ),
                            ],
                          ),
                        ),
                      ),
                      SizedBox(height: Spacing.medium(context)),
                      FilledButton.icon(
                        onPressed: () {
                          context.go(AppRoutePaths.settings);
                        },
                        icon: const Icon(Icons.tune),
                        label: Text(l10n.globalSettingsScreenTitle),
                        style: FilledButton.styleFrom(
                          padding: Spacing.buttonPadding(context),
                        ),
                      ),
                      SizedBox(height: Spacing.small(context)),
                      // Кнопка API-ключей
                      FilledButton.icon(
                        onPressed: () {
                          context.go(AppRoutePaths.profileApiKeys);
                        },
                        icon: const Icon(Icons.vpn_key),
                        label: Text(l10n.apiKeysManage),
                        style: FilledButton.styleFrom(
                          padding: Spacing.buttonPadding(context),
                        ),
                      ),
                      SizedBox(height: Spacing.small(context)),
                      // Кнопка обновить данные
                      OutlinedButton.icon(
                        onPressed: () {
                          ref
                              .read(authControllerProvider.notifier)
                              .refreshUser();
                        },
                        icon: const Icon(Icons.refresh),
                        label: Text(l10n.refreshData),
                        style: OutlinedButton.styleFrom(
                          padding: Spacing.buttonPadding(context),
                        ),
                      ),
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
                      SizedBox(height: Spacing.medium(context)),
                      ElevatedButton(
                        onPressed: () {
                          ref
                              .read(authControllerProvider.notifier)
                              .refreshUser();
                        },
                        child: Text(l10n.retry),
                      ),
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
  final BuildContext context;
  final IconData icon;
  final String label;
  final String value;
  final Color? valueColor;

  const _InfoRow({
    required this.context,
    required this.icon,
    required this.label,
    required this.value,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: Spacing.iconSize(context)),
        SizedBox(width: Spacing.mini(context)),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                label,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
              ),
              SizedBox(height: Spacing.mini(context) / 2),
              Text(
                value,
                style: Theme.of(
                  context,
                ).textTheme.bodyLarge?.copyWith(color: valueColor),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
