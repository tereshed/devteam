import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/settings/domain/models/llm_provider_model.dart';
import 'package:frontend/features/settings/presentation/widgets/llm_providers_section.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Sprint 15.34 — widget-тесты LLMProvidersSection.
void main() {
  Future<void> pump(
    WidgetTester tester, {
    required List<LLMProviderModel> providers,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          llmProvidersListProvider.overrideWith((ref) async => providers),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(body: LLMProvidersSection()),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('empty state — shows "no providers" message and Add button',
      (tester) async {
    await pump(tester, providers: const []);
    final l10n = AppLocalizations.of(
      tester.element(find.byType(LLMProvidersSection)),
    )!;
    expect(find.text(l10n.llmProvidersEmpty), findsOneWidget);
    expect(find.widgetWithText(FilledButton, l10n.llmProvidersAdd), findsOneWidget);
  });

  testWidgets('non-empty state — renders ListTile per provider with kind/model',
      (tester) async {
    await pump(tester, providers: const [
      LLMProviderModel(
        id: '11111111-1111-1111-1111-111111111111',
        name: 'OpenRouter prod',
        kind: 'openrouter',
        baseURL: 'https://openrouter.ai/api/v1',
        defaultModel: 'openrouter/auto',
        enabled: true,
      ),
      LLMProviderModel(
        id: '22222222-2222-2222-2222-222222222222',
        name: 'Local Ollama',
        kind: 'ollama',
        defaultModel: 'llama3',
        enabled: false,
      ),
    ]);

    expect(find.text('OpenRouter prod'), findsOneWidget);
    expect(find.text('openrouter • openrouter/auto'), findsOneWidget);
    expect(find.text('Local Ollama'), findsOneWidget);
    // Один enabled (зелёный check), один disabled (серый cancel).
    expect(find.byIcon(Icons.check_circle), findsOneWidget);
    expect(find.byIcon(Icons.cancel), findsOneWidget);
  });
}
