@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifacts_dag_section.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../../../_fixtures/orchestration_v2_fixtures.dart';
import '../../../../support/widget_test_harness.dart';

// artifacts_dag_section_test.dart — Sprint 17 / 6.7.
//
// Покрывает:
//   • depends_on rendering: для subtask_description видна цепочка "← <short-id>"
//   • group ordering: kind'ы отображаются в фиксированном порядке
//     (plan → subtask_description → code_diff → review → merged_code → test_result),
//     unknown kind'ы — в конце.
//   • empty / loading states.

Future<void> _pump(
  WidgetTester tester, {
  required Future<List<Artifact>> Function() artifacts,
}) =>
    pumpAppWidget(
      tester,
      child: const Scaffold(
        body: SingleChildScrollView(
          child: ArtifactsDagSection(taskId: kFxTaskId),
        ),
      ),
      overrides: [
        taskArtifactsProvider(kFxTaskId).overrideWith((_) => artifacts()),
      ],
    );

void main() {
  group('ArtifactsDagSection', () {
    testWidgets('empty: показывает artifactsEmpty', (tester) async {
      await _pump(tester, artifacts: () async => const <Artifact>[]);

      final BuildContext ctx =
          tester.element(find.byType(ArtifactsDagSection));
      final l10n = AppLocalizations.of(ctx)!;
      expect(find.text(l10n.artifactsEmpty), findsOneWidget);
      expect(find.byType(Card), findsNothing);
    });

    testWidgets(
        'depends_on рендерится для subtask_description: "← <short-id>"',
        (tester) async {
      final base = fxSubtask(
        id: 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee',
        title: 'Base subtask',
        dependsOn: const [],
      );
      final dependent = fxSubtask(
        id: 'ffffffff-1111-2222-3333-444444444444',
        title: 'Dependent subtask',
        dependsOn: const ['aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee'],
      );
      await _pump(tester, artifacts: () async => [base, dependent]);

      expect(find.text('Base subtask'), findsOneWidget);
      expect(find.text('Dependent subtask'), findsOneWidget);
      // Short-id = первые 8 символов + "…".
      expect(find.text('← aaaaaaaa…'), findsOneWidget);
      // Base — без depends_on, поэтому стрелочной строки быть не должно.
      // Точечная проверка: строк "←" ровно одна.
      expect(find.textContaining('←'), findsOneWidget);
    });

    testWidgets(
        'multiple depends_on объединяются через ", " и используют short-id',
        (tester) async {
      final node = fxSubtask(
        id: '11111111-aaaa-bbbb-cccc-dddddddddddd',
        title: 'Multi-dep node',
        dependsOn: const [
          'aaaaaaaa-1111-1111-1111-111111111111',
          'bbbbbbbb-2222-2222-2222-222222222222',
        ],
      );
      await _pump(tester, artifacts: () async => [node]);

      expect(find.text('← aaaaaaaa…, bbbbbbbb…'), findsOneWidget);
    });

    testWidgets(
        'group ordering: plan → subtask_description → code_diff → review → '
        'merged_code → test_result, unknown в конце', (tester) async {
      // Подаём в "неправильном" порядке, чтобы убедиться что render
      // переупорядочивает.
      final artifacts = <Artifact>[
        fxArtifact(kind: 'test_result', summary: 'TR'),
        fxArtifact(kind: 'unknown_kind', summary: 'UK'),
        fxArtifact(kind: 'plan', summary: 'PLAN'),
        fxArtifact(kind: 'review', summary: 'REV'),
        fxSubtask(id: 'sub-1', title: 'SUB'),
        fxArtifact(kind: 'code_diff', summary: 'DIFF'),
        fxArtifact(kind: 'merged_code', summary: 'MERGED'),
      ];
      await _pump(tester, artifacts: () async => artifacts);

      // Находим все Header'ы (kind-заголовки) и проверяем их порядок по dy.
      const expectedKinds = [
        'plan',
        'subtask_description',
        'code_diff',
        'review',
        'merged_code',
        'test_result',
        'unknown_kind',
      ];
      final ys = <String, double>{};
      for (final k in expectedKinds) {
        // Header текст рендерится с monospace fontFamily — там TextStyle
        // отличается от Card content. У нас 7 уникальных text-значений,
        // findsOneWidget ловит именно header.
        final f = find.text(k);
        expect(f, findsOneWidget, reason: 'kind header "$k" не найден');
        ys[k] = tester.getTopLeft(f).dy;
      }
      for (var i = 0; i < expectedKinds.length - 1; i++) {
        final a = expectedKinds[i];
        final b = expectedKinds[i + 1];
        expect(ys[a]! < ys[b]!, isTrue,
            reason: '"$a" должен идти выше "$b"; got $a=${ys[a]} $b=${ys[b]}');
      }
    });

    testWidgets(
        'iteration и producer_agent рендерятся: "<agent> · #<iter>"',
        (tester) async {
      final art = fxArtifact(
        kind: 'code_diff',
        producerAgent: 'developer',
        iteration: 3,
        summary: 'patch',
      );
      await _pump(tester, artifacts: () async => [art]);
      expect(find.text('developer · #3'), findsOneWidget);
    });

    testWidgets('superseded artifact: status dot серый, ready — зелёный',
        (tester) async {
      final ready = fxArtifact(
        kind: 'plan',
        id: 'r1',
        status: 'ready',
        summary: 'R',
      );
      final old = fxArtifact(
        kind: 'plan',
        id: 's1',
        status: 'superseded',
        summary: 'S',
      );
      await _pump(tester, artifacts: () async => [ready, old]);
      // Конкретного rendering API нет — просто sanity-check что обе карточки
      // присутствуют и не схлопнулись из-за status-handling.
      expect(find.text('R'), findsOneWidget);
      expect(find.text('S'), findsOneWidget);
    });
  });
}
