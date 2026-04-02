import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

class LandingScreen extends ConsumerWidget {
  const LandingScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDesktop = Responsive.isDesktop(context);
    final l10n = AppLocalizations.of(context)!;
    final authState = ref.watch(authControllerProvider);
    final isLoggedIn = authState.value != null;

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.appTitle),
        actions: [
          if (isLoggedIn)
            ElevatedButton(
              onPressed: () => context.go('/dashboard'),
              child: Text(l10n.goToDashboard),
            )
          else ...[
            TextButton(
              onPressed: () => context.go('/login'),
              child: Text(l10n.login),
            ),
            const SizedBox(width: 16),
            ElevatedButton(
              onPressed: () => context.go('/register'),
              child: Text(l10n.getStarted),
            ),
          ],
          const SizedBox(width: 24),
        ],
      ),
      body: SingleChildScrollView(
        child: Column(
          children: [
            // Hero Section
            Container(
              width: double.infinity,
              padding: EdgeInsets.symmetric(
                vertical: isDesktop ? 120 : 60,
                horizontal: 24,
              ),
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topLeft,
                  end: Alignment.bottomRight,
                  colors: [
                    theme.colorScheme.primaryContainer.withValues(alpha: 0.3),
                    theme.colorScheme.surface,
                  ],
                ),
              ),
              child: AdaptiveContainer(
                child: Column(
                  children: [
                    Text(
                      l10n.landingTitle,
                      style: theme.textTheme.displayLarge?.copyWith(
                        fontWeight: FontWeight.bold,
                        height: 1.1,
                      ),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 24),
                    Text(
                      l10n.landingSubtitle,
                      style: theme.textTheme.headlineSmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 48),
                    Wrap(
                      spacing: 16,
                      runSpacing: 16,
                      alignment: WrapAlignment.center,
                      children: [
                        ElevatedButton(
                          onPressed: () => context.go(
                            isLoggedIn ? '/dashboard' : '/register',
                          ),
                          style: ElevatedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 32,
                              vertical: 20,
                            ),
                            textStyle: theme.textTheme.titleLarge,
                          ),
                          child: Text(
                            isLoggedIn ? l10n.goToDashboard : l10n.startForFree,
                          ),
                        ),
                        OutlinedButton(
                          onPressed: () {
                            // TODO: Scroll to features or docs
                          },
                          style: OutlinedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 32,
                              vertical: 20,
                            ),
                            textStyle: theme.textTheme.titleLarge,
                          ),
                          child: Text(l10n.learnMore),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),

            // Features Placeholder
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 80, horizontal: 24),
              child: AdaptiveContainer(
                child: Column(
                  children: [
                    Text(l10n.whyWibe, style: theme.textTheme.displaySmall),
                    const SizedBox(height: 48),
                    Wrap(
                      spacing: 24,
                      runSpacing: 24,
                      alignment: WrapAlignment.center,
                      children: [
                        _FeatureCard(
                          icon: Icons.speed,
                          title: l10n.featurePerformanceTitle,
                          description: l10n.featurePerformanceDesc,
                        ),
                        _FeatureCard(
                          icon: Icons.security,
                          title: l10n.featureSecurityTitle,
                          description: l10n.featureSecurityDesc,
                        ),
                        _FeatureCard(
                          icon: Icons.devices,
                          title: l10n.featureCrossPlatformTitle,
                          description: l10n.featureCrossPlatformDesc,
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _FeatureCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;

  const _FeatureCard({
    required this.icon,
    required this.title,
    required this.description,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 300,
      padding: const EdgeInsets.all(24),
      decoration: BoxDecoration(
        color: Theme.of(
          context,
        ).colorScheme.surfaceContainerHighest.withValues(alpha: 0.3),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: Theme.of(context).colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 40, color: Theme.of(context).colorScheme.primary),
          const SizedBox(height: 16),
          Text(
            title,
            style: Theme.of(
              context,
            ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Text(description, style: Theme.of(context).textTheme.bodyLarge),
        ],
      ),
    );
  }
}
