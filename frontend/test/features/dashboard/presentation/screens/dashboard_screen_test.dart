// @Tags(['widget'])
//
// Widget-тесты hub-экрана /dashboard: 4 stat-карточки и empty-state
// блока «Последние задачи». Покрывает AC задачи 1.4.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/features/dashboard/presentation/providers/dashboard_summary_provider.dart';
import 'package:frontend/features/dashboard/presentation/screens/dashboard_screen.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

const _user = UserModel(id: 'u1', email: 'kostya@example.com', role: 'admin');

class _FakeAuthController extends AuthController {
  @override
  Future<UserModel?> build() async => _user;
}

Future<void> _pump(
  WidgetTester tester, {
  required AsyncValue<DashboardSummary> summary,
  required AsyncValue<List<TaskListItemModel>> tasks,
}) async {
  final router = GoRouter(
    initialLocation: '/dashboard',
    routes: [
      GoRoute(
        path: '/dashboard',
        builder: (_, _) => const DashboardScreen(),
      ),
      GoRoute(
        path: '/projects',
        builder: (_, _) => const Text('PROJECTS_PAGE'),
      ),
      GoRoute(
        path: '/admin/agents-v2',
        builder: (_, _) => const Text('AGENTS_PAGE'),
      ),
      GoRoute(
        path: '/integrations/llm',
        builder: (_, _) => const Text('LLM_PAGE'),
      ),
      GoRoute(
        path: '/integrations/git',
        builder: (_, _) => const Text('GIT_PAGE'),
      ),
    ],
  );

  tester.view.physicalSize = const Size(1400, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        dashboardSummaryProvider.overrideWith((ref) async {
          if (summary.hasError) {
            // ignore: only_throw_errors
            throw summary.error!;
          }
          return summary.value!;
        }),
        dashboardRecentTasksProvider.overrideWith(
          (ref) async => tasks.value ?? const [],
        ),
        // Стабы для зависимых провайдеров (вдруг activated в фоне).
        agentsV2ListProvider.overrideWith(
          (ref) async => const AgentV2Page(total: 0, items: [], limit: 0, offset: 0),
        ),
      ],
      child: MaterialApp.router(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: router,
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  group('DashboardScreen', () {
    testWidgets('рендерит 4 stat-карточки и empty-state recent tasks',
        (tester) async {
      const summary = AsyncData(
        DashboardSummary(
          projectsActive: 3,
          projectsTotal: 5,
          agentsTotal: 7,
          llmConnected: 0,
          gitConnected: 0,
        ),
      );

      await _pump(
        tester,
        summary: summary,
        tasks: const AsyncData(<TaskListItemModel>[]),
      );

      // 4 заголовка карточек (равны nav-лейблам).
      expect(find.text('Projects'), findsWidgets);
      expect(find.text('Agents'), findsWidgets);
      expect(find.text('LLM providers'), findsWidgets);
      expect(find.text('Git providers'), findsWidgets);

      // Значения отрисованы.
      expect(find.text('3 active'), findsOneWidget);
      expect(find.text('5 projects in total'), findsOneWidget);
      expect(find.text('7 agents'), findsOneWidget);

      // CTA «Manage» появляется на каждой карточке.
      expect(find.text('Manage'), findsNWidgets(4));

      // Empty-state блока recent tasks.
      expect(find.text('No tasks yet'), findsOneWidget);
    });

    testWidgets('summary в ошибке: показывает «—» вместо нулей',
        (tester) async {
      await _pump(
        tester,
        // Эмулируем «не было успешного fetch'а + сейчас ошибка» — провайдер
        // отдаёт AsyncError, valueOrNull == null.
        summary: AsyncError(Exception('boom'), StackTrace.current),
        tasks: const AsyncData(<TaskListItemModel>[]),
      );

      // Прочерк должен появиться в каждой из 4 карточек.
      expect(find.text('—'), findsNWidgets(4));
      // Никаких «0 active» / «0 projects in total» в этом сценарии.
      expect(find.text('0 active'), findsNothing);
      expect(find.text('No projects in total'), findsNothing);
    });

    testWidgets('тап по карточке Projects ведёт на /projects',
        (tester) async {
      const summary = AsyncData(
        DashboardSummary(
          projectsActive: 0,
          projectsTotal: 0,
          agentsTotal: 0,
          llmConnected: 0,
          gitConnected: 0,
        ),
      );

      await _pump(
        tester,
        summary: summary,
        tasks: const AsyncData(<TaskListItemModel>[]),
      );

      // Карточка projects кликабельна — тап по её CTA «Manage».
      await tester.tap(find.text('Manage').first);
      await tester.pumpAndSettle();
      expect(find.text('PROJECTS_PAGE'), findsOneWidget);
    });
  });
}

