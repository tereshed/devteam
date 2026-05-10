@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/presentation/utils/agent_role_display.dart';
import 'package:frontend/features/tasks/domain/models/task_model.dart';
import 'package:frontend/features/tasks/presentation/utils/task_status_display.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_card.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';
import 'package:intl/intl.dart';

import '../../../projects/helpers/test_wrappers.dart';
import '../../helpers/task_fixtures.dart';

Widget _wrapTaskCard(Widget child, {TextScaler? textScaler}) {
  return wrapSimple(
    child,
    locale: const Locale('ru'),
    textScaler: textScaler,
    scrollableBody: true,
  );
}

void main() {
  final l10nRu = AppLocalizationsRu();

  testWidgets('shows taskCardAgentLine when assignedAgent is set', (tester) async {
    const name = 'Имя агента';
    final task = makeTaskListItemFixture(
      id: '11111111-1111-1111-1111-111111111111',
      title: 'Заголовок',
      assignedAgent: const AgentSummaryModel(
        id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        name: name,
        role: 'developer',
      ),
    );
    final roleLabel = agentRoleLabel(l10nRu, 'developer');
    final expectedLine = l10nRu.taskCardAgentLine(name, roleLabel);

    await tester.pumpWidget(
      _wrapTaskCard(TaskCard(task: task, onTap: () {})),
    );
    await tester.pumpAndSettle();

    expect(find.text(expectedLine), findsOneWidget);
  });

  testWidgets('shows taskCardUnassigned when assignedAgent is null', (tester) async {
    final task = makeTaskListItemFixture(
      id: '22222222-2222-2222-2222-222222222222',
      title: 'Без агента',
    );

    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.taskCardUnassigned), findsOneWidget);
  });

  testWidgets('12.10 RU-smoke: taskCardUnassigned', (tester) async {
    final task = makeTaskListItemFixture(
      id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
      title: 'RU smoke title',
    );
    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    await tester.pumpAndSettle();
    expect(find.text(l10nRu.taskCardUnassigned), findsOneWidget);
  });

  testWidgets('chip label matches taskStatusLabel from wire status', (tester) async {
    final task = makeTaskListItemFixture(
      id: '55555555-5555-5555-5555-555555555555',
      title: 'Статус',
      status: 'review',
    );
    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    await tester.pumpAndSettle();
    expect(find.text(taskStatusLabel(l10nRu, 'review')), findsOneWidget);
  });

  testWidgets('updatedAt line uses DateFormat matching dense flag', (tester) async {
    final updatedAt = DateTime.utc(2026, 7, 8, 15, 5);
    final task = makeTaskListItemFixture(
      id: '33333333-3333-3333-3333-333333333333',
      title: 'Время',
      updatedAt: updatedAt,
    );

    final local = updatedAt.toLocal();
    final longFmt = DateFormat.yMMMd('ru').add_jm().format(local);
    final shortFmt = DateFormat.MMMd('ru').add_Hm().format(local);
    expect(shortFmt, isNot(equals(longFmt)),
        reason: 'Kanban and list formats must differ for this locale');

    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    await tester.pumpAndSettle();
    expect(
      find.text(l10nRu.taskCardUpdatedAt(longFmt)),
      findsOneWidget,
    );

    await tester.pumpWidget(
      _wrapTaskCard(TaskCard(task: task, dense: true, onTap: () {})),
    );
    await tester.pumpAndSettle();
    expect(
      find.text(l10nRu.taskCardUpdatedAt(shortFmt)),
      findsOneWidget,
    );
  });

  testWidgets('onTap wires InkWell; null onTap omits InkWell', (tester) async {
    final task = makeTaskListItemFixture(
      id: '44444444-4444-4444-4444-444444444444',
      title: 'Тап',
    );

    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    expect(find.byType(InkWell), findsOneWidget);

    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task)));
    expect(find.byType(InkWell), findsNothing);
  });

  testWidgets('onTap callback fires on tap', (tester) async {
    var taps = 0;
    final task = makeTaskListItemFixture(
      id: 'faaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
      title: 'Tap callback',
    );
    await tester.pumpWidget(
      _wrapTaskCard(
        TaskCard(task: task, onTap: () => taps++),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Tap callback'));
    await tester.pump();
    expect(taps, 1);
  });

  testWidgets('Semantics: button when onTap; not a button without onTap', (tester) async {
    final handle = tester.ensureSemantics();
    try {
      final task = makeTaskListItemFixture(
        id: 'c0000001-0000-4000-8000-000000000001',
        title: 'Sem',
      );

      await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
      await tester.pumpAndSettle();

      final rootButtonSemantics = find.descendant(
        of: find.byType(TaskCard),
        matching: find.byWidgetPredicate(
          (w) =>
              w is Semantics &&
              w.properties.button == true &&
              w.properties.label == 'Sem',
        ),
      );
      expect(rootButtonSemantics, findsOneWidget);
      final sem = tester.getSemantics(rootButtonSemantics).getSemanticsData();
      // ignore: deprecated_member_use — flagsCollection API ещё не везде одинаков в CI
      expect(sem.hasFlag(SemanticsFlag.isButton), isTrue);

      await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task)));
      await tester.pumpAndSettle();
      expect(rootButtonSemantics, findsNothing);

      // Заголовок по-прежнему в дереве семантики как текст (без обёртки-кнопки).
      final titleText = find.text('Sem');
      expect(titleText, findsOneWidget);
      final titleSem = tester.getSemantics(titleText).getSemanticsData();
      // ignore: deprecated_member_use — flagsCollection API ещё не везде одинаков в CI
      expect(titleSem.hasFlag(SemanticsFlag.isButton), isFalse);
    } finally {
      handle.dispose();
    }
  });

  testWidgets('dense sets title maxLines to 3; default is 4', (tester) async {
    final longTitle = List.filled(20, 'word').join(' ');
    final task = makeTaskListItemFixture(
      id: 'c0000002-0000-4000-8000-000000000002',
      title: longTitle,
    );

    await tester.pumpWidget(_wrapTaskCard(TaskCard(task: task, onTap: () {})));
    await tester.pumpAndSettle();
    expect(tester.widget<Text>(find.text(longTitle)).maxLines, 4);

    await tester.pumpWidget(
      _wrapTaskCard(TaskCard(task: task, dense: true, onTap: () {})),
    );
    await tester.pumpAndSettle();
    expect(tester.widget<Text>(find.text(longTitle)).maxLines, 3);
  });

  testWidgets('TextScaler 2.0 does not throw or overflow error', (tester) async {
    final task = makeTaskListItemFixture(
      id: '66666666-6666-6666-6666-666666666666',
      title: 'Очень длинный заголовок задачи ' * 5,
    );

    await tester.pumpWidget(
      _wrapTaskCard(
        TaskCard(
          task: task,
          onTap: () {},
        ),
        textScaler: const TextScaler.linear(2.0),
      ),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
  });
}
