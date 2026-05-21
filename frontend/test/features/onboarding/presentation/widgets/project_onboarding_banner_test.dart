import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/onboarding/data/onboarding_providers.dart';
import 'package:frontend/features/onboarding/domain/onboarding_state.dart';
import 'package:frontend/features/onboarding/presentation/widgets/project_onboarding_banner.dart';
import 'package:frontend/l10n/app_localizations.dart';

Widget _wrap({required ProjectOnboardingState state}) {
  return ProviderScope(
    overrides: [
      projectOnboardingStateProvider('p1').overrideWithValue(state),
    ],
    child: const MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(
        body: ProjectOnboardingBanner(projectId: 'p1'),
      ),
    ),
  );
}

void main() {
  group('ProjectOnboardingBanner', () {
    testWidgets('hidden when loading', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const ProjectOnboardingState(loading: true),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.settings_suggest), findsNothing);
    });

    testWidgets('hidden when both agents configured', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const ProjectOnboardingState(
          loading: false,
          orchestratorConfigured: true,
          routerConfigured: true,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.settings_suggest), findsNothing);
    });

    testWidgets('shows hint when orchestrator not configured', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const ProjectOnboardingState(
          loading: false,
          orchestratorConfigured: false,
          routerConfigured: true,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.settings_suggest), findsOneWidget);
      expect(find.text('Configure agents'), findsOneWidget);
    });

    testWidgets('shows hint when router not configured', (tester) async {
      await tester.pumpWidget(_wrap(
        state: const ProjectOnboardingState(
          loading: false,
          orchestratorConfigured: true,
          routerConfigured: false,
        ),
      ));
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.settings_suggest), findsOneWidget);
    });
  });
}
