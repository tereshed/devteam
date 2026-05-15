@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/router_decision_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/router_timeline_section.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../../../_fixtures/orchestration_v2_fixtures.dart';
import '../../../../support/widget_test_harness.dart';

// router_timeline_section_test.dart — Sprint 17 / 6.7.
//
// Покрывает:
//   • outcome chip: рендерится только когда decision.outcome != null
//     (DONE-step имеет outcome 'success' / 'failed' / etc.)
//   • multi-agent decisions: parallel fan-out — chosenAgents склеиваются
//     через " · " в monospace-строке.
//   • empty / error / loading states.

Future<void> _pump(
  WidgetTester tester, {
  required Future<List<RouterDecision>> Function() decisions,
}) =>
    pumpAppWidget(
      tester,
      child: const Scaffold(
        body: SingleChildScrollView(
          child: RouterTimelineSection(taskId: kFxTaskId),
        ),
      ),
      overrides: [
        taskRouterDecisionsProvider(kFxTaskId).overrideWith((_) => decisions()),
      ],
    );

void main() {
  group('RouterTimelineSection', () {
    testWidgets('empty: показывает routerTimelineEmpty', (tester) async {
      await _pump(tester, decisions: () async => const <RouterDecision>[]);
      final BuildContext ctx =
          tester.element(find.byType(RouterTimelineSection));
      final l10n = AppLocalizations.of(ctx)!;
      expect(find.text(l10n.routerTimelineEmpty), findsOneWidget);
      expect(find.byType(Card), findsNothing);
    });

    testWidgets(
        'outcome chip: рендерится только когда outcome присутствует '
        '(done-step)', (tester) async {
      final inProgress = fxDecision(
        id: 'd1',
        stepNo: 1,
        agents: const ['planner'],
        reason: 'split task',
        // outcome == null → in-progress step.
      );
      final done = fxDecision(
        id: 'd2',
        stepNo: 2,
        agents: const ['merger'],
        outcome: 'success',
        reason: 'all subtasks merged',
      );
      await _pump(tester, decisions: () async => [inProgress, done]);

      // Outcome-chip есть только для done.
      expect(find.widgetWithText(Chip, 'success'), findsOneWidget);
      // Если бы chip был у обоих, мы бы увидели 'success' дважды или
      // другой текст. Дополнительно проверяем количество Chip-ов = 1.
      expect(find.byType(Chip), findsOneWidget);
    });

    testWidgets(
        'multi-agent decision: chosenAgents объединяются через " · "',
        (tester) async {
      final parallel = fxDecision(
        id: 'd-multi',
        stepNo: 3,
        agents: const ['developer', 'developer', 'tester'],
        reason: 'fan-out: 2 dev + 1 tester в параллель',
      );
      await _pump(tester, decisions: () async => [parallel]);

      // Sublabel — monospace, формат "a · b · c".
      expect(find.text('developer · developer · tester'), findsOneWidget);
      expect(find.text('fan-out: 2 dev + 1 tester в параллель'), findsOneWidget);
    });

    testWidgets('chosenAgents == []: показывает placeholder "—"',
        (tester) async {
      final empty = fxDecision(
        id: 'd-noop',
        stepNo: 4,
        agents: const <String>[],
        outcome: 'success',
        reason: 'noop — already done',
      );
      await _pump(tester, decisions: () async => [empty]);
      expect(find.text('—'), findsOneWidget);
    });

    testWidgets('step_no рендерится в CircleAvatar', (tester) async {
      final decisions = <RouterDecision>[
        fxDecision(id: 'd1', stepNo: 1),
        fxDecision(id: 'd7', stepNo: 7, agents: const ['reviewer']),
      ];
      await _pump(tester, decisions: () async => decisions);

      // Цифры step внутри CircleAvatar.
      expect(find.text('1'), findsOneWidget);
      expect(find.text('7'), findsOneWidget);
    });

    testWidgets('reason отображается под header-строкой', (tester) async {
      final d = fxDecision(
        id: 'd1',
        agents: const ['planner'],
        reason: 'нужно сначала составить план',
      );
      await _pump(tester, decisions: () async => [d]);
      expect(find.text('нужно сначала составить план'), findsOneWidget);
    });

    testWidgets('error: показывает dataLoadError', (tester) async {
      await _pump(
        tester,
        decisions: () async => throw StateError('boom'),
      );
      final BuildContext ctx =
          tester.element(find.byType(RouterTimelineSection));
      final l10n = AppLocalizations.of(ctx)!;
      expect(find.textContaining(l10n.dataLoadError), findsOneWidget);
      expect(find.textContaining('boom'), findsOneWidget);
    });
  });
}
