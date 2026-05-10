@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/team/presentation/widgets/agent_card.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';

import '../../../projects/helpers/test_wrappers.dart';

/// [InkWell] зоны тапа карточки: обёртка вокруг основного [Padding] с отступами `EdgeInsets.all(16)`.
/// Ищем именно её, чтобы тест не зависел от других [InkWell] в дочерних виджетах.
Finder _agentCardSurfaceInkWell() {
  return find.descendant(
    of: find.byType(AgentCard),
    matching: find.byWidgetPredicate(
      (w) =>
          w is InkWell &&
          w.child is Padding &&
          (w.child! as Padding).padding == const EdgeInsets.all(16),
    ),
  );
}

Widget _wrapAgentCard(Widget child, {TextScaler? textScaler}) {
  return wrapSimple(
    child,
    locale: const Locale('ru'),
    textScaler: textScaler,
    scrollableBody: true,
  );
}

AgentModel _baseAgent({
  String id = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
  String name = 'Имя агента',
  String role = 'developer',
  String? model = 'claude-opus-4-7',
  String? promptName,
  String? codeBackend,
  bool isActive = true,
}) {
  return AgentModel(
    id: id,
    name: name,
    role: role,
    model: model,
    promptName: promptName,
    codeBackend: codeBackend,
    isActive: isActive,
  );
}

void main() {
  final l10nRu = AppLocalizationsRu();

  testWidgets('active agent shows teamAgentActive text', (tester) async {
    final agent = _baseAgent(isActive: true);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.teamAgentActive), findsWidgets);
  });

  testWidgets('inactive agent shows teamAgentInactive text', (tester) async {
    final agent = _baseAgent(isActive: false);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.teamAgentInactive), findsWidgets);
  });

  testWidgets('empty name shows teamAgentNameUnset', (tester) async {
    final agent = _baseAgent(name: '   ');
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.teamAgentNameUnset), findsOneWidget);
  });

  testWidgets('model null shows teamAgentModelUnset', (tester) async {
    final agent = _baseAgent(model: null);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.teamAgentModelUnset), findsOneWidget);
  });

  testWidgets('shows promptName and codeBackend when set', (tester) async {
    const prompt = 'System prompt X';
    const backend = 'claude-code';
    final agent = _baseAgent(promptName: prompt, codeBackend: backend);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(prompt), findsOneWidget);
    expect(find.text(backend), findsOneWidget);
  });

  testWidgets('shows promptName only when codeBackend unset', (tester) async {
    const prompt = 'PromptSoloOnly';
    final agent = _baseAgent(promptName: prompt, codeBackend: null);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(prompt), findsOneWidget);
    expect(find.text('BackendGhostNeverSet'), findsNothing);
  });

  testWidgets('shows codeBackend only when promptName unset', (tester) async {
    const backend = 'aider-custom-wire';
    final agent = _baseAgent(promptName: null, codeBackend: backend);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(backend), findsOneWidget);
    expect(find.text('PromptGhostNeverSet'), findsNothing);
  });

  testWidgets('omits prompt and code_backend lines when unset', (tester) async {
    const ghostPrompt = 'GhostPromptLineAbsent';
    const ghostBackend = 'GhostBackendLineAbsent';
    final agent = _baseAgent(promptName: null, codeBackend: null);
    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    await tester.pumpAndSettle();

    expect(find.text(ghostPrompt), findsNothing);
    expect(find.text(ghostBackend), findsNothing);
    expect(find.text('claude-code'), findsNothing);
  });

  testWidgets('onTap wires InkWell; null onTap omits InkWell', (tester) async {
    final agent = _baseAgent();
    var called = false;

    await tester.pumpWidget(
      _wrapAgentCard(AgentCard(agent: agent, onTap: () => called = true)),
    );
    expect(_agentCardSurfaceInkWell(), findsOneWidget);

    await tester.tap(_agentCardSurfaceInkWell());
    await tester.pump();
    expect(called, isTrue);

    await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
    expect(_agentCardSurfaceInkWell(), findsNothing);
  });

  testWidgets('Semantics: button when onTap; not a button without onTap',
      (tester) async {
    final handle = tester.ensureSemantics();
    try {
      const displayName = 'Семантика';
      final agent = _baseAgent(name: displayName);

      await tester.pumpWidget(
        _wrapAgentCard(AgentCard(agent: agent, onTap: () {})),
      );
      await tester.pumpAndSettle();

      final rootButtonSemantics = find.descendant(
        of: find.byType(AgentCard),
        matching: find.byWidgetPredicate(
          (w) =>
              w is Semantics &&
              w.properties.button == true &&
              w.properties.label == displayName,
        ),
      );
      expect(rootButtonSemantics, findsOneWidget);
      final sem = tester.getSemantics(rootButtonSemantics).getSemanticsData();
      // ignore: deprecated_member_use — flagsCollection API ещё не везде одинаков в CI
      expect(sem.hasFlag(SemanticsFlag.isButton), isTrue);

      await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
      await tester.pumpAndSettle();
      expect(rootButtonSemantics, findsNothing);

      final titleText = find.text(displayName);
      expect(titleText, findsOneWidget);
      final titleSem = tester.getSemantics(titleText).getSemanticsData();
      // ignore: deprecated_member_use
      expect(titleSem.hasFlag(SemanticsFlag.isButton), isFalse);
    } finally {
      handle.dispose();
    }
  });

  testWidgets('activity chip has Semantics label for active', (tester) async {
    final handle = tester.ensureSemantics();
    try {
      final agent = _baseAgent(isActive: true);
      await tester.pumpWidget(_wrapAgentCard(AgentCard(agent: agent)));
      await tester.pumpAndSettle();

      final chipSemantics = find.descendant(
        of: find.byType(AgentCard),
        matching: find.byWidgetPredicate(
          (w) =>
              w is Semantics &&
              w.properties.label == l10nRu.teamAgentActive &&
              w.properties.button != true,
        ),
      );
      expect(chipSemantics, findsWidgets);
    } finally {
      handle.dispose();
    }
  });
}
