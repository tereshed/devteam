// @Tags(['widget'])
//
// Widget-тесты [AppShell]: смена layout'а на breakpoint'ах 600 и 1200dp.
// Покрывает AC задачи 1.2 из docs/tasks/ui_refactoring/tasks-breakdown.md.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/widgets/app_shell.dart';
import 'package:frontend/features/auth/domain/models.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';

const _user = UserModel(
  id: 'u1',
  email: 'kostya@example.com',
  role: 'admin',
);

GoRouter _router() => GoRouter(
      initialLocation: '/dashboard',
      routes: [
        ShellRoute(
          builder: (context, state, child) => AppShell(
            location: state.matchedLocation,
            child: child,
          ),
          routes: [
            GoRoute(
              path: '/dashboard',
              builder: (_, _) => const Text('CONTENT'),
            ),
            GoRoute(
              path: '/projects',
              builder: (_, _) => const Text('PROJECTS'),
            ),
          ],
        ),
      ],
    );

Future<void> _pumpAt(
  WidgetTester tester, {
  required Size size,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
      ],
      child: MaterialApp.router(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        routerConfig: _router(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

class _FakeAuthController extends AuthController {
  @override
  Future<UserModel?> build() async => _user;
}

void main() {
  group('AppShell', () {
    testWidgets('desktop ≥1200: rail развёрнут с лейблами', (tester) async {
      await _pumpAt(tester, size: const Size(1400, 900));

      expect(find.text('CONTENT'), findsOneWidget);
      // Развёрнутый rail показывает локализованные лейблы пунктов.
      expect(find.text('Overview'), findsWidgets);
      expect(find.text('Projects'), findsWidgets);
      // Группы тоже видны.
      expect(find.text('Resources'), findsOneWidget);
      // Без Drawer — burger-кнопки нет.
      expect(find.byTooltip('Open navigation menu'), findsNothing);
    });

    testWidgets('tablet 600..1200: rail свёрнут (только иконки)', (
      tester,
    ) async {
      await _pumpAt(tester, size: const Size(900, 900));
      expect(find.text('CONTENT'), findsOneWidget);
      // В свёрнутом виде заголовки групп («Resources» и т. д.) не рендерятся.
      expect(find.text('Resources'), findsNothing);
      // Пункт «Overview» как text не виден (только иконка + tooltip).
      // Tooltip ищется отдельно — он создаётся динамически и виден на hover.
      expect(find.byTooltip('Overview'), findsOneWidget);
    });

    testWidgets('mobile <600: Drawer вместо rail', (tester) async {
      await _pumpAt(tester, size: const Size(400, 800));
      expect(find.text('CONTENT'), findsOneWidget);
      // Изначально drawer закрыт.
      expect(find.byType(Drawer), findsNothing);
      // Открываем drawer.
      await tester.tap(find.byTooltip('Open navigation menu'));
      await tester.pumpAndSettle();
      expect(find.byType(Drawer), findsOneWidget);
      expect(find.text('Overview'), findsWidgets);
    });
  });
}
