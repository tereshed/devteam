import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/onboarding/presentation/widgets/onboarding_banner.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  Widget wrap(Widget child) {
    return MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: child),
    );
  }

  group('OnboardingBanner', () {
    testWidgets('renders message, action button, and dismiss button',
        (tester) async {
      var tapped = false;
      await tester.pumpWidget(wrap(
        OnboardingBanner(
          icon: Icons.info,
          message: 'Test message',
          actionLabel: 'Go',
          onAction: () => tapped = true,
        ),
      ));

      expect(find.text('Test message'), findsOneWidget);
      expect(find.text('Go'), findsOneWidget);
      expect(find.byIcon(Icons.info), findsOneWidget);
      expect(find.byIcon(Icons.close), findsOneWidget);

      await tester.tap(find.text('Go'));
      expect(tapped, isTrue);
    });

    testWidgets('dismiss hides the banner', (tester) async {
      await tester.pumpWidget(wrap(
        OnboardingBanner(
          icon: Icons.info,
          message: 'Dismissable banner',
          actionLabel: 'Action',
          onAction: () {},
        ),
      ));

      expect(find.text('Dismissable banner'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.close));
      await tester.pumpAndSettle();

      expect(find.text('Dismissable banner'), findsNothing);
    });
  });
}
