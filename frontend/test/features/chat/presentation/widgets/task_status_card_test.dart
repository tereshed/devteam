import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_card.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_visuals.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../helpers/test_wrappers.dart';

/// Технические заголовки в тесте смены [ValueKey], не UI-литералы продукта.
const kTaskStatusSwapCardTitleA = 'Card A';
const kTaskStatusSwapCardTitleB = 'Card B';

void main() {
  const kTid = '660e8400-e29b-41d4-a716-446655440001';

  group('M3 категории по иконкам', () {
    testWidgets('Active: in_progress → autorenew', (tester) async {
      await tester.pumpWidget(
        wrapTaskStatusRu(
          const TaskStatusCard(
            key: ValueKey(kTid),
            taskId: kTid,
            status: 'in_progress',
            title: 'X',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.autorenew), findsOneWidget);
    });

    testWidgets('Success: completed → check_circle', (tester) async {
      await tester.pumpWidget(
        wrapTaskStatusRu(
          const TaskStatusCard(
            key: ValueKey(kTid),
            taskId: kTid,
            status: 'completed',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.check_circle), findsOneWidget);
    });

    testWidgets('Error: failed → error icon', (tester) async {
      await tester.pumpWidget(
        wrapTaskStatusRu(
          const TaskStatusCard(
            key: ValueKey(kTid),
            taskId: kTid,
            status: 'failed',
            errorMessage: 'boom',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.error), findsOneWidget);
    });

    testWidgets('Stopped: paused → pause_circle', (tester) async {
      await tester.pumpWidget(
        wrapTaskStatusRu(
          const TaskStatusCard(
            key: ValueKey(kTid),
            taskId: kTid,
            status: 'paused',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.pause_circle), findsOneWidget);
    });

    testWidgets('Unknown: bogus_xyz → pause_circle, не autorenew', (tester) async {
      await tester.pumpWidget(
        wrapTaskStatusRu(
          const TaskStatusCard(
            key: ValueKey(kTid),
            taskId: kTid,
            status: 'bogus_xyz',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.pause_circle), findsOneWidget);
      expect(find.byIcon(Icons.autorenew), findsNothing);
    });
  });

  group('параметризованный smoke + Semantics (ru)', () {
    for (final status in [...kNormativeTaskStatuses, 'bogus_xyz']) {
      testWidgets('статус в Semantics: $status', (tester) async {
        const tid = '770e8400-e29b-41d4-a716-446655440099';
        await tester.pumpWidget(
          wrapTaskStatusRu(
            TaskStatusCard(
              key: const ValueKey(tid),
              taskId: tid,
              status: status,
              title: 'My title',
            ),
          ),
        );
        await tester.pumpAndSettle();

        final ctx = tester.element(find.byKey(const ValueKey(tid)));
        final l10n = AppLocalizations.of(ctx)!;
        final statusLbl = taskStatusLabel(l10n, status);
        final data = tester.getSemantics(find.byKey(const ValueKey(tid))).getSemanticsData();
        expect(data.label, allOf(contains('My title'), contains(statusLbl)));
        if (status == 'bogus_xyz') {
          expect(find.byIcon(Icons.pause_circle), findsOneWidget);
          expect(find.byIcon(Icons.autorenew), findsNothing);
        }
      });
    }
  });

  testWidgets('неизвестный bogus_xyz: подпись taskStatusUnknownStatus', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey(kTid),
          taskId: kTid,
          status: 'bogus_xyz',
        ),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(tester.element(find.byKey(const ValueKey(kTid))))!;
    expect(find.text(l10n.taskStatusUnknownStatus), findsOneWidget);
  });

  testWidgets('l10n: каждый taskStatus* не пустой и не равен сырому status', (tester) async {
    late AppLocalizations l10n;
    await tester.pumpWidget(
      wrapChatMaterialAppLite(
        locale: const Locale('ru'),
        home: Builder(
          builder: (ctx) {
            l10n = AppLocalizations.of(ctx)!;
            return const SizedBox();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    for (final s in kNormativeTaskStatuses) {
      final label = taskStatusLabel(l10n, s);
      expect(label, isNotEmpty);
      expect(label, isNot(equals(s)));
    }
    expect(l10n.taskStatusUnknownStatus, isNotEmpty);
  });

  testWidgets('onOpen == null: нет InkWell; Semantics isButton false', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey(kTid),
          taskId: kTid,
          status: 'pending',
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.descendant(
        of: find.byKey(const ValueKey(kTid)),
        matching: find.byType(InkWell),
      ),
      findsNothing,
    );
    final sem = tester.getSemantics(find.byKey(const ValueKey(kTid))).getSemanticsData();
    // ignore: deprecated_member_use — flagsCollection API ещё не везде одинаков в CI
    expect(sem.hasFlag(SemanticsFlag.isButton), isFalse);
  });

  testWidgets('onOpen != null: InkWell + Semantics isButton true', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        TaskStatusCard(
          key: const ValueKey(kTid),
          taskId: kTid,
          status: 'pending',
          onOpen: (_) {},
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.descendant(
        of: find.byKey(const ValueKey(kTid)),
        matching: find.byType(InkWell),
      ),
      findsOneWidget,
    );
    final sem = tester.getSemantics(find.byKey(const ValueKey(kTid))).getSemanticsData();
    // ignore: deprecated_member_use
    expect(sem.hasFlag(SemanticsFlag.isButton), isTrue);
  });

  testWidgets('agentRole == null: нет · и нет локализованной роли', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey(kTid),
          taskId: kTid,
          status: 'planning',
        ),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(tester.element(find.byKey(const ValueKey(kTid))))!;
    expect(find.textContaining('·'), findsNothing);
    expect(
      find.textContaining(taskCardAgentRoleLabel(l10n, TaskCardAgentRole.developer)),
      findsNothing,
    );
  });

  testWidgets('agentRole != null: есть · и роль', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey('660e8400-e29b-41d4-a716-446655440002'),
          taskId: '660e8400-e29b-41d4-a716-446655440002',
          status: 'planning',
          agentRole: TaskCardAgentRole.developer,
        ),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byKey(const ValueKey('660e8400-e29b-41d4-a716-446655440002'))),
    )!;
    expect(find.textContaining('·'), findsOneWidget);
    expect(
      find.textContaining(taskCardAgentRoleLabel(l10n, TaskCardAgentRole.developer)),
      findsOneWidget,
    );
  });

  testWidgets('длинный title: не бросает, maxLines 2', (tester) async {
    final long = List.filled(120, 'word').join(' ');
    await tester.pumpWidget(
      wrapTaskStatusRu(
        TaskStatusCard(
          key: const ValueKey('660e8400-e29b-41d4-a716-446655440003'),
          taskId: '660e8400-e29b-41d4-a716-446655440003',
          status: 'review',
          title: long,
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
    final twoLineTitles = tester
        .widgetList<Text>(find.byType(Text))
        .where((t) => t.maxLines == 2);
    expect(twoLineTitles, isNotEmpty);
  });

  testWidgets('TextScaler 2.0: строится без overflow exception', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey(kTid),
          taskId: kTid,
          status: 'testing',
          title: 'Scaled',
        ),
        textScaler: const TextScaler.linear(2),
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('RTL: строится без исключений', (tester) async {
    await tester.pumpWidget(
      wrapTaskStatusRu(
        const TaskStatusCard(
          key: ValueKey(kTid),
          taskId: kTid,
          status: 'changes_requested',
        ),
        direction: TextDirection.rtl,
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  testWidgets('матрица status × ThemeMode light/dark', (tester) async {
    for (final mode in [ThemeMode.light, ThemeMode.dark]) {
      for (final status in [...kNormativeTaskStatuses, 'bogus_xyz']) {
        await tester.pumpWidget(
          wrapTaskStatusRu(
            TaskStatusCard(
              key: const ValueKey('660e8400-e29b-41d4-a716-446655440001'),
              taskId: '660e8400-e29b-41d4-a716-446655440001',
              status: status,
            ),
            themeMode: mode,
          ),
        );
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        expect(find.byType(TaskStatusCard), findsOneWidget);
      }
    }
  });

  testWidgets('ValueKey: смена порядка двух карточек не путает заголовки', (tester) async {
    await tester.pumpWidget(
      wrapChatMaterialAppLite(
        locale: const Locale('ru'),
        home: const _SwapKeysHarness(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text(kTaskStatusSwapCardTitleA), findsOneWidget);
    expect(find.text(kTaskStatusSwapCardTitleB), findsOneWidget);
    await tester.tap(find.text('swap'));
    await tester.pumpAndSettle();
    expect(find.text(kTaskStatusSwapCardTitleA), findsOneWidget);
    expect(find.text(kTaskStatusSwapCardTitleB), findsOneWidget);
  });

  testWidgets('golden in_progress light', (tester) async {
    await tester.pumpWidget(
      wrapChatMaterialAppLite(
        locale: const Locale('ru'),
        theme: ThemeData.light(useMaterial3: true),
        themeMode: ThemeMode.light,
        home: const Center(
          child: RepaintBoundary(
            key: ValueKey('golden_rb'),
            child: SizedBox(
              width: 380,
              height: 220,
              child: TaskStatusCard(
                key: ValueKey(kTid),
                taskId: kTid,
                status: 'in_progress',
                title: 'Golden sample',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await expectLater(
      find.byKey(const ValueKey('golden_rb')),
      matchesGoldenFile('goldens/task_status_card_in_progress_light.png'),
    );
  });

  testWidgets('golden in_progress dark', (tester) async {
    await tester.pumpWidget(
      wrapChatMaterialAppLite(
        locale: const Locale('ru'),
        theme: ThemeData.light(useMaterial3: true),
        darkTheme: ThemeData.dark(useMaterial3: true),
        themeMode: ThemeMode.dark,
        home: const Center(
          child: RepaintBoundary(
            key: ValueKey('golden_rb'),
            child: SizedBox(
              width: 380,
              height: 220,
              child: TaskStatusCard(
                key: ValueKey(kTid),
                taskId: kTid,
                status: 'in_progress',
                title: 'Golden sample',
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await expectLater(
      find.byKey(const ValueKey('golden_rb')),
      matchesGoldenFile('goldens/task_status_card_in_progress_dark.png'),
    );
  });
}

class _SwapKeysHarness extends StatefulWidget {
  const _SwapKeysHarness();

  @override
  State<_SwapKeysHarness> createState() => _SwapKeysHarnessState();
}

class _SwapKeysHarnessState extends State<_SwapKeysHarness> {
  bool swapped = false;

  @override
  Widget build(BuildContext context) {
    const a = TaskStatusCard(
      key: ValueKey('aaa11111-1111-1111-1111-111111111111'),
      taskId: 'aaa11111-1111-1111-1111-111111111111',
      status: 'completed',
      title: kTaskStatusSwapCardTitleA,
    );
    const b = TaskStatusCard(
      key: ValueKey('bbb22222-2222-2222-2222-222222222222'),
      taskId: 'bbb22222-2222-2222-2222-222222222222',
      status: 'pending',
      title: kTaskStatusSwapCardTitleB,
    );
    return Column(
      children: [
        TextButton(
          onPressed: () => setState(() => swapped = !swapped),
          child: const Text('swap'),
        ),
        if (!swapped) ...[a, b] else ...[b, a],
      ],
    );
  }
}
