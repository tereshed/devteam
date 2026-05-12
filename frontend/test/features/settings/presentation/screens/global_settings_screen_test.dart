import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/app_route_paths.dart';
import 'package:frontend/features/settings/data/claude_code_auth_providers.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/settings/domain/global_settings_backend_gate.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:frontend/features/settings/presentation/screens/global_settings_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

/// Sprint 15 — global settings теперь TabBar(3); тесты пушат «DevTeam» вкладку,
/// прежде чем проверять старые ожидания (blocker path, api keys button).
List<Override> _defaultSpringtimeOverrides() => [
      llmProvidersListProvider.overrideWith((ref) async => <LLMProviderModel>[]),
      claudeCodeAuthStatusProvider
          .overrideWith((ref) async => const ClaudeCodeAuthStatus(connected: false)),
    ];

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
          overrides: _defaultSpringtimeOverrides(),
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
          overrides: _defaultSpringtimeOverrides(),
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

  Future<void> _openDevTeamTab(WidgetTester tester) async {
    final l10n = AppLocalizations.of(
      tester.element(find.byType(GlobalSettingsScreen)),
    )!;
    await tester.tap(find.text(l10n.globalSettingsTabDevTeam));
    await tester.pumpAndSettle();
  }

  testWidgets('B1: DevTeam-вкладка содержит blocker path и нет редакторов ключей', (
    WidgetTester tester,
  ) async {
    await pumpStub(tester);
    await _openDevTeamTab(tester);
    expect(find.byType(GlobalSettingsScreen), findsOneWidget);
    expect(find.text(globalSettingsBackendBlockerDocsPath), findsOneWidget);
    expect(find.byType(TextField), findsNothing);
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
    expect(find.text(l10n.globalSettingsTabLLMProviders), findsOneWidget);
    expect(find.text(l10n.globalSettingsTabClaudeCode), findsOneWidget);
    await _openDevTeamTab(tester);
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
    await _openDevTeamTab(tester);

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
