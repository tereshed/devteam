@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_repository.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_exceptions.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/admin/agents_v2/presentation/screens/agents_v2_list_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';

import '../../../../../_fixtures/orchestration_v2_fixtures.dart';
import '../../../../../support/widget_test_harness.dart';

// agents_v2_list_screen_test.dart — Sprint 17 / 6.7.
//
// Покрывает три состояния FutureProvider'а: empty / data / error, плюс кнопку
// refresh в AppBar (invalidate → повторный вызов repo.list).

class _MockRepo extends Mock implements AgentsV2Repository {}

GoRouter _testRouter() => GoRouter(
      initialLocation: '/admin/agents-v2',
      routes: [
        GoRoute(
          path: '/admin/agents-v2',
          builder: (_, _) => const AgentsV2ListScreen(),
        ),
        GoRoute(
          // Стаб для context.go(...detail). Реальный экран в этом тесте не
          // важен — нам нужно только не падать на навигации.
          path: '/admin/agents-v2/:id',
          builder: (_, state) => Scaffold(
            body: Text('detail-${state.pathParameters['id']}'),
          ),
        ),
      ],
    );

Future<void> _pump(
  WidgetTester tester, {
  required List<Override> overrides,
  Size? screenSize,
}) =>
    pumpAppWidgetWithRouter(
      tester,
      routerConfig: _testRouter(),
      overrides: overrides,
      screenSize: screenSize,
    );

void main() {
  group('AgentsV2ListScreen', () {
    testWidgets('empty: показывает agentsV2Empty', (tester) async {
      final repo = _MockRepo();
      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
          agentsV2ListProvider.overrideWith(
            (_) async => fxAgentPage(const <AgentV2>[]),
          ),
        ],
      );

      final BuildContext ctx = tester.element(find.byType(AgentsV2ListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      expect(find.text(l10n.agentsV2Empty), findsOneWidget);
      // FAB всегда виден — даже в empty-состоянии оператор должен иметь
      // возможность создать первого агента.
      expect(find.byType(FloatingActionButton), findsOneWidget);
      expect(find.byType(ListTile), findsNothing);
    });

    testWidgets('data: рендерит N tile, llm/sandbox subtitle различается',
        (tester) async {
      final llm = fxAgent(
        id: 'aaa',
        name: 'planner-claude',
        role: 'planner',
        executionKind: 'llm',
        model: 'claude-sonnet-4-6',
      );
      final sandbox = fxAgent(
        id: 'bbb',
        name: 'dev-claude-code',
        role: 'developer',
        executionKind: 'sandbox',
        codeBackend: 'claude-code',
      );
      final repo = _MockRepo();
      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
          agentsV2ListProvider.overrideWith(
            (_) async => fxAgentPage([llm, sandbox]),
          ),
        ],
      );

      expect(find.text('planner-claude'), findsOneWidget);
      expect(find.text('dev-claude-code'), findsOneWidget);
      // Subtitle = "<role> · <kindLabel> · <model или code_backend>".
      expect(find.text('planner · LLM · claude-sonnet-4-6'), findsOneWidget);
      expect(find.text('developer · Sandbox · claude-code'), findsOneWidget);
      expect(find.byType(ListTile), findsNWidgets(2));
    });

    testWidgets('error: показывает dataLoadError', (tester) async {
      final repo = _MockRepo();
      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
          agentsV2ListProvider.overrideWith(
            (_) async =>
                throw AgentV2ApiException('boom', statusCode: 500),
          ),
        ],
      );

      final BuildContext ctx = tester.element(find.byType(AgentsV2ListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      // Текст содержит префикс из l10n + наш message. textContaining,
      // т.к. форматирование "$prefix: $err" зависит от ToString().
      expect(find.textContaining(l10n.dataLoadError), findsOneWidget);
      expect(find.textContaining('boom'), findsOneWidget);
    });

    testWidgets('inactive agent: trailing icon = pause_circle_outline',
        (tester) async {
      final repo = _MockRepo();
      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
          agentsV2ListProvider.overrideWith(
            (_) async => fxAgentPage([fxAgent(isActive: false)]),
          ),
        ],
      );

      expect(find.byIcon(Icons.pause_circle_outline), findsOneWidget);
      expect(find.byIcon(Icons.check_circle), findsNothing);
    });

    testWidgets('golden: data state — список из двух агентов', (tester) async {
      // Golden фиксирует общую вёрстку экрана: AppBar + tile'ы LLM/sandbox +
      // FAB. Цель — поймать неожиданные drift'ы (изменения отступов, иконок,
      // typography) при работе над соседними фичами. Не покрывает динамику —
      // только статичный data-render.
      //
      // Стабильность:
      //   • Фиксированный screenSize (1.0 devicePixelRatio).
      //   • RepaintBoundary с явным размером — снапшот только этой области.
      //   • Detimestamp'ные данные: вся изменчивость (createdAt) скрыта в
      //     fixture-builder'е (фиксированная DateTime.utc(2026, 5, 15, 12)).
      //
      // Регенерация при намеренном изменении UI:
      //     flutter test --update-goldens path/to/this_file.dart
      final llm = fxAgent(
        id: 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
        name: 'planner-claude',
        role: 'planner',
        executionKind: 'llm',
        model: 'claude-sonnet-4-6',
      );
      final sandbox = fxAgent(
        id: 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
        name: 'dev-claude-code',
        role: 'developer',
        executionKind: 'sandbox',
        codeBackend: 'claude-code',
      );
      final inactive = fxAgent(
        id: 'cccccccc-cccc-cccc-cccc-cccccccccccc',
        name: 'tester-aider',
        role: 'tester',
        executionKind: 'sandbox',
        codeBackend: 'aider',
        isActive: false,
      );
      final repo = _MockRepo();
      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
          agentsV2ListProvider.overrideWith(
            (_) async => fxAgentPage([llm, sandbox, inactive]),
          ),
        ],
        screenSize: const Size(800, 600),
      );
      await expectLater(
        find.byType(AgentsV2ListScreen),
        matchesGoldenFile('goldens/agents_v2_list_data.png'),
      );
    });

    testWidgets('refresh AppBar-кнопка инвалидирует провайдер', (tester) async {
      // Используем override через ref.read repo.list, чтобы убедиться что
      // refresh приводит к новому вызову. Override провайдера константой
      // не позволил бы это проверить — нужен реальный fetch через repo.
      final repo = _MockRepo();
      var callCount = 0;
      when(() => repo.list(
            onlyActive: any(named: 'onlyActive'),
            executionKind: any(named: 'executionKind'),
            role: any(named: 'role'),
            nameLike: any(named: 'nameLike'),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async {
        callCount++;
        return fxAgentPage([fxAgent()]);
      });

      await _pump(
        tester,
        overrides: [
          agentsV2RepositoryProvider.overrideWithValue(repo),
        ],
      );

      expect(callCount, 1);
      // Tap refresh icon в AppBar.
      await tester.tap(find.byIcon(Icons.refresh));
      await tester.pumpAndSettle();
      expect(callCount, 2);
    });
  });
}
