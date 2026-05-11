import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/features/settings/domain/global_settings_backend_gate.dart';
import 'package:frontend/features/settings/presentation/screens/global_settings_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

void main() {
  Future<void> pumpStub(
    WidgetTester tester, {
    Locale locale = const Locale('en'),
    GoRouter? router,
  }) async {
    if (router != null) {
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          child: MaterialApp.router(
            locale: locale,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            routerConfig: router,
          ),
        ),
      );
    } else {
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          child: MaterialApp(
            locale: locale,
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const GlobalSettingsScreen(),
          ),
        ),
      );
    }
    await tester.pumpAndSettle();
  }

  testWidgets('B1: stub shows blocker path, no TextField, no Save', (
    WidgetTester tester,
  ) async {
    await pumpStub(tester);
    expect(find.byType(GlobalSettingsScreen), findsOneWidget);
    expect(find.text(globalSettingsBackendBlockerDocsPath), findsOneWidget);
    expect(find.byType(TextField), findsNothing);
    expect(find.byType(ElevatedButton), findsNothing);
    expect(find.byType(FilledButton), findsNothing);
    final l10n = AppLocalizations.of(
      tester.element(find.byType(GlobalSettingsScreen)),
    )!;
    expect(find.text(l10n.save), findsNothing);
  });

  testWidgets('B2: Russian locale uses arb strings', (WidgetTester tester) async {
    await pumpStub(tester, locale: const Locale('ru'));
    final ctx = tester.element(find.byType(GlobalSettingsScreen));
    final l10n = AppLocalizations.of(ctx)!;
    expect(find.text(l10n.globalSettingsScreenTitle), findsOneWidget);
    expect(find.text(l10n.globalSettingsStubIntro), findsOneWidget);
    expect(find.text(l10n.globalSettingsBlockedByLabel), findsOneWidget);
  });

  testWidgets('B3: Application API keys button navigates to profile api-keys', (
    WidgetTester tester,
  ) async {
    const stubMarker = '__test_api_keys_route_marker__';
    final router = GoRouter(
      initialLocation: AppRoutePaths.settings,
      routes: [
        GoRoute(
          path: AppRoutePaths.settings,
          builder: (context, state) => const GlobalSettingsScreen(),
        ),
        GoRoute(
          path: AppRoutePaths.profileApiKeys,
          builder: (context, state) =>
              const Scaffold(body: Text(stubMarker)),
        ),
      ],
    );
    await pumpStub(tester, router: router);

    final l10n = AppLocalizations.of(
      tester.element(find.byType(GlobalSettingsScreen)),
    )!;
    final apiKeysButton = find.widgetWithText(
      OutlinedButton,
      l10n.globalSettingsOpenDevTeamApiKeys,
    );
    await tester.ensureVisible(apiKeysButton);
    await tester.pumpAndSettle();
    await tester.tap(apiKeysButton);
    await tester.pumpAndSettle();

    expect(find.text(stubMarker), findsOneWidget);
    expect(router.state.uri.path, AppRoutePaths.profileApiKeys);
  });
}
