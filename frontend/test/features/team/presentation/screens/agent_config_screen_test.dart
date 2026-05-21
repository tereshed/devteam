import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/mcp_registry_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/team/data/agent_settings_providers.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';
import 'package:frontend/features/team/presentation/screens/agent_config_screen.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/l10n/app_localizations.dart';

const _agentId = 'agent-test-001';
final _now = DateTime.utc(2026, 5, 20);

AgentV2 _makeAgent({
  String role = 'developer',
  String executionKind = 'llm',
  bool isActive = true,
  bool internalMcpEnabled = false,
  String? providerKind = 'anthropic',
  String? model = 'claude-sonnet-4-20250514',
  double? temperature = 0.7,
}) =>
    AgentV2(
      id: _agentId,
      name: 'Test Agent',
      role: role,
      roleDescription: 'Test role',
      executionKind: executionKind,
      isActive: isActive,
      internalMcpEnabled: internalMcpEnabled,
      createdAt: _now,
      updatedAt: _now,
      providerKind: providerKind,
      model: model,
      temperature: temperature,
    );

const _defaultSettings = AgentSettingsModel(agentID: _agentId);

Future<void> _pump(
  WidgetTester tester, {
  AgentV2? agent,
  AgentSettingsModel settings = _defaultSettings,
  List<MCPServerRegistryModel> mcpServers = const [],
}) async {
  final a = agent ?? _makeAgent();
  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: [
        agentV2DetailProvider(a.id).overrideWith((ref) async => a),
        agentSettingsProvider(a.id).overrideWith((ref) async => settings),
        mcpRegistryListProvider.overrideWith((ref) async => mcpServers),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: AgentConfigScreen(agentId: a.id),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  group('AgentConfigScreen', () {
    testWidgets('renders all sections for LLM agent', (tester) async {
      await _pump(tester);
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text('Test Agent'), findsOneWidget);
      expect(find.text(l10n.agentConfigActiveLabel), findsOneWidget);
      expect(find.text(l10n.agentConfigRoleSectionTitle), findsWidgets);
      expect(find.text(l10n.agentConfigTypeSectionTitle), findsOneWidget);
      expect(find.text(l10n.agentConfigLLMSectionTitle), findsOneWidget);
      expect(find.text(l10n.agentConfigMCPSectionTitle), findsOneWidget);
      expect(find.text(l10n.agentConfigSkillsSectionTitle), findsOneWidget);
    });

    testWidgets('hides LLM settings section for sandbox agent', (tester) async {
      await _pump(tester, agent: _makeAgent(executionKind: 'sandbox'));
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigLLMSectionTitle), findsNothing);
      expect(find.text(l10n.agentConfigRoleSectionTitle), findsWidgets);
      expect(find.text(l10n.agentConfigMCPSectionTitle), findsOneWidget);
    });

    testWidgets('active toggle changes subtitle', (tester) async {
      await _pump(tester, agent: _makeAgent(isActive: true));
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigActiveOn), findsOneWidget);
      expect(find.text(l10n.agentConfigActiveOff), findsNothing);

      await tester.tap(find.byType(SwitchListTile).first);
      await tester.pumpAndSettle();

      expect(find.text(l10n.agentConfigActiveOff), findsOneWidget);
      expect(find.text(l10n.agentConfigActiveOn), findsNothing);
    });

    testWidgets('role is read-only for auto-created roles (assistant)',
        (tester) async {
      await _pump(tester, agent: _makeAgent(role: 'assistant'));
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigRoleReadOnly), findsOneWidget);
      expect(find.text('assistant'), findsWidgets);
      // The InputDecorator shows role text, not a DropdownButtonFormField for role
      expect(find.byType(InputDecorator), findsWidgets);
    });

    testWidgets('role is read-only for auto-created roles (orchestrator)',
        (tester) async {
      await _pump(tester, agent: _makeAgent(role: 'orchestrator'));
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigRoleReadOnly), findsOneWidget);
    });

    testWidgets('role dropdown is editable for non-auto-created roles',
        (tester) async {
      await _pump(tester, agent: _makeAgent(role: 'developer'));

      expect(find.byType(DropdownButtonFormField<String>), findsWidgets);
    });

    testWidgets('save button appears after toggling active', (tester) async {
      await _pump(tester);
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigSaveButton), findsNothing);

      await tester.tap(find.byType(SwitchListTile).first);
      await tester.pumpAndSettle();

      expect(find.text(l10n.agentConfigSaveButton), findsOneWidget);
    });

    testWidgets('shows loading indicator while agent loads', (tester) async {
      final completer = Completer<AgentV2>();
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            agentV2DetailProvider(_agentId)
                .overrideWith((ref) => completer.future),
            agentSettingsProvider(_agentId)
                .overrideWith((ref) async => _defaultSettings),
            mcpRegistryListProvider.overrideWith((ref) async => []),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AgentConfigScreen(agentId: _agentId),
          ),
        ),
      );
      await tester.pump();

      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      completer.complete(_makeAgent());
      await tester.pumpAndSettle();
    });

    testWidgets('shows error when agent load fails', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            agentV2DetailProvider(_agentId).overrideWith(
              (ref) => Future<AgentV2>.error(Exception('network error')),
            ),
            agentSettingsProvider(_agentId)
                .overrideWith((ref) async => _defaultSettings),
            mcpRegistryListProvider.overrideWith((ref) async => []),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: const AgentConfigScreen(agentId: _agentId),
          ),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.textContaining('network error'), findsOneWidget);
    });

    testWidgets('type section shows segmented button', (tester) async {
      await _pump(tester);

      expect(find.byType(SegmentedButton<String>), findsOneWidget);
    });

    testWidgets('LLM settings shows provider dropdown and model field',
        (tester) async {
      await _pump(tester, agent: _makeAgent(executionKind: 'llm'));
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigProviderLabel), findsOneWidget);
      expect(find.text(l10n.agentConfigModelLabel), findsOneWidget);
    });

    testWidgets('MCP section shows DevTeam MCP toggle', (tester) async {
      await _pump(tester);
      final l10n = requireAppLocalizations(
        tester.element(find.byType(AgentConfigScreen)),
        where: 'AgentConfigScreen_test',
      );

      expect(find.text(l10n.agentConfigDevTeamMCP), findsOneWidget);
      expect(find.text(l10n.agentConfigDevTeamMCPDesc), findsOneWidget);
    });
  });
}
