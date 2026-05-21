import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/onboarding/data/onboarding_providers.dart';
import 'package:frontend/features/onboarding/domain/onboarding_state.dart';
import 'package:frontend/features/onboarding/presentation/widgets/dashboard_onboarding_banner.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

Widget _wrap({required OnboardingState state}) {
  final router = GoRouter(
    initialLocation: '/test',
    routes: [
      GoRoute(
        path: '/test',
        builder: (_, _) => const Scaffold(
          body: DashboardOnboardingBanner(),
        ),
      ),
      GoRoute(
        path: '/integrations/llm',
        builder: (_, _) => const Text('LLM_PAGE'),
      ),
      GoRoute(
        path: '/settings',
        builder: (_, _) => const Text('SETTINGS_PAGE'),
      ),
    ],
  );

  return ProviderScope(
    overrides: [
      onboardingStateProvider.overrideWithValue(state),
    ],
    child: MaterialApp.router(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      routerConfig: router,
    ),
  );
}

void main() {
  group('DashboardOnboardingBanner', () {
    testWidgets('hidden when loading', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const OnboardingState(loading: true),
      ));
      await tester.pumpAndSettle();

      expect(find.byType(DashboardOnboardingBanner), findsOneWidget);
      expect(find.byIcon(Icons.power_settings_new), findsNothing);
      expect(find.byIcon(Icons.smart_toy_outlined), findsNothing);
    });

    testWidgets('hidden when assistant is configured', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const OnboardingState(
          loading: false,
          hasLlmProviders: true,
          assistantConfigured: true,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.power_settings_new), findsNothing);
      expect(find.byIcon(Icons.smart_toy_outlined), findsNothing);
    });

    testWidgets('shows LLM provider hint when no providers', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const OnboardingState(
          loading: false,
          hasLlmProviders: false,
          assistantConfigured: false,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.power_settings_new), findsOneWidget);

      await tester.tap(find.text('Go to settings'));
      await tester.pumpAndSettle();
      expect(find.text('LLM_PAGE'), findsOneWidget);
    });

    testWidgets('shows assistant config hint when providers exist',
        (tester) async {
      await tester.pumpWidget(_wrap(
        state: const OnboardingState(
          loading: false,
          hasLlmProviders: true,
          assistantConfigured: false,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.smart_toy_outlined), findsOneWidget);

      await tester.tap(find.text('Go to settings'));
      await tester.pumpAndSettle();
      expect(find.text('SETTINGS_PAGE'), findsOneWidget);
    });
  });
}
