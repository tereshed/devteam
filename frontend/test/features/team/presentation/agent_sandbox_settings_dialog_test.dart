import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/settings/data/llm_providers_providers.dart';
import 'package:frontend/features/team/data/agent_settings_providers.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';
import 'package:frontend/features/team/presentation/widgets/agent_sandbox_settings_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Sprint 15.34 — widget-тесты вкладок agent advanced settings.
void main() {
  Future<void> pumpAndOpen(
    WidgetTester tester, {
    required AgentSettingsModel current,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          agentSettingsProvider(current.agentID)
              .overrideWith((ref) async => current),
          llmProvidersListProvider.overrideWith((ref) async => const []),
        ],
        child: MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Builder(
            builder: (ctx) => Scaffold(
              body: TextButton(
                onPressed: () => showAgentSandboxSettingsDialog(
                  ctx,
                  agentID: current.agentID,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();
  }

  testWidgets('shows 4 tabs with localized titles', (tester) async {
    await pumpAndOpen(
      tester,
      current: const AgentSettingsModel(agentID: 'agent-1'),
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byType(Dialog)),
    )!;
    expect(find.text(l10n.agentSandboxSettingsTabProvider), findsOneWidget);
    expect(find.text(l10n.agentSandboxSettingsTabMCP), findsOneWidget);
    expect(find.text(l10n.agentSandboxSettingsTabSkills), findsOneWidget);
    expect(find.text(l10n.agentSandboxSettingsTabPermissions), findsOneWidget);
  });

  testWidgets('permissions tab: existing allow patterns render as chips',
      (tester) async {
    await pumpAndOpen(
      tester,
      current: const AgentSettingsModel(
        agentID: 'agent-2',
        sandboxPermissions: {
          'allow': ['Read', 'Bash(go test:*)'],
          'defaultMode': 'acceptEdits',
        },
      ),
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byType(Dialog)),
    )!;
    // переключаемся на вкладку permissions.
    await tester.tap(find.text(l10n.agentSandboxSettingsTabPermissions));
    await tester.pumpAndSettle();
    expect(find.text('Read'), findsOneWidget);
    expect(find.text('Bash(go test:*)'), findsOneWidget);
  });
}
