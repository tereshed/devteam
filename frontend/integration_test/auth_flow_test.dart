// Phase 3 §Task 3.2 — Auth flow integration tests (см. docs/integration-tests-plan.md).
//
// Что покрываем:
//   1. UI-регистрация: /register → form → submit → redirect на /dashboard.
//   2. UI-логин: REST-регистрация → /login → form → submit → /dashboard.
//   3. Логаут: pumpFreshAuthedApp → /profile → tap logout → редирект на /.
//   4. Неавторизованный доступ к protected: /projects без токена → редирект на /.
//
// **Никаких LLM-вызовов** в этом файле нет (auth — это auth/me + auth/login).
// Цена прогона: близко к нулю. Cost-leak guard: Phase 3 wrapper (Go
// `frontend_e2e_test.go`) проверит `SELECT COUNT(*) FROM llm_logs` до/после
// и упадёт, если backend получил хоть один LLM-вызов.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/auth/presentation/screens/login_screen.dart';
import 'package:frontend/features/auth/presentation/screens/register_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/seed_creds.dart';
import 'test_support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'register UI: fill form → POST /auth/register → land on /dashboard',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // НЕ инжектим overrides — нужен реальный путь через AuthController.
      // resetTestStorage чистит TokenStorage от прошлого прогона.
      await pumpFreshTestApp(tester);

      // На лендинге; идём на /register через go_router (а не через UI-кнопку
      // лендинга — её локализация может меняться, нам нужен экран регистрации).
      GoRouter.of(anyScaffoldContext(tester)).go('/register');
      await tester.pumpAndSettle(const Duration(seconds: 1));

      await expectEventually(
        tester,
        find.byType(RegisterScreen),
        reason: 'register screen visible',
      );

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // 3 TextFormField: email, password, confirm. Порядок зафиксирован в
      // register_screen.dart — менять только синхронно с этим тестом.
      final fields = find.byType(TextFormField);
      expect(fields, findsAtLeast(3), reason: 'email + password + confirm');

      final email = uniqueTestEmail('auth-ui-reg');
      const password = 'Password123!';
      await tester.enterText(fields.at(0), email);
      await tester.enterText(fields.at(1), password);
      await tester.enterText(fields.at(2), password);
      await tester.pump();

      // ElevatedButton с текстом l10n.register — единственный submit на экране.
      await tester.tap(find.widgetWithText(ElevatedButton, l10n.register));
      // POST /auth/register → POST /auth/me → redirect /dashboard. Сетевые I/O
      // pumpAndSettle не ждёт — bounded loop.
      await pumpForSeconds(tester, 10);

      await expectEventually(
        tester,
        find.text(l10n.dashboardHubSubtitle),
        timeout: const Duration(seconds: 10),
        reason: 'dashboard reached after registration',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );

  testWidgets(
    'login UI: REST-seeded user → fill /login form → /dashboard',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // Регистрируем юзера через REST — он останется в БД и нам нужны его креды.
      final creds = await registerSeedUser(prefix: 'auth-ui-login');

      // НЕ инжектим — пройдём реальный путь /login.
      await pumpFreshTestApp(tester);

      GoRouter.of(anyScaffoldContext(tester)).go('/login');
      await tester.pumpAndSettle(const Duration(seconds: 1));

      await expectEventually(
        tester,
        find.byType(LoginScreen),
        reason: 'login screen visible',
      );

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      final fields = find.byType(TextFormField);
      expect(fields, findsAtLeast(2), reason: 'email + password');
      await tester.enterText(fields.at(0), creds.email);
      await tester.enterText(fields.at(1), creds.password);
      await tester.pump();

      await tester.tap(find.widgetWithText(ElevatedButton, l10n.login));
      await pumpForSeconds(tester, 10);

      await expectEventually(
        tester,
        find.text(l10n.dashboardHubSubtitle),
        timeout: const Duration(seconds: 10),
        reason: 'dashboard reached after login',
      );
      // На /dashboard видим welcome с email seed-юзера — sanity, что dio
      // получил access_token и /auth/me вернул правильного юзера.
      await expectEventually(
        tester,
        find.text(l10n.dashboardWelcomeUser(creds.email)),
        timeout: const Duration(seconds: 5),
        reason: 'welcome with seed-user email',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );

  testWidgets(
    'unauthorized redirect: /projects without token → landing page',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // Чистое состояние без токена.
      await pumpFreshTestApp(tester);

      // /projects — protected; rootRouterRedirect должен вернуть «/».
      GoRouter.of(anyScaffoldContext(tester)).go('/projects');
      await pumpForSeconds(tester, 3);

      final currentLocation = GoRouter.of(
        anyScaffoldContext(tester),
      ).routerDelegate.currentConfiguration.uri.path;
      expect(
        currentLocation,
        equals('/'),
        reason: 'authGuard must redirect unauthenticated user to landing',
      );
    },
    timeout: const Timeout(Duration(minutes: 1)),
  );

  testWidgets(
    'logout: authed → /profile → tap logout → landing page',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      await pumpFreshAuthedApp(tester, prefix: 'auth-ui-logout');

      GoRouter.of(anyScaffoldContext(tester)).go('/profile');
      await pumpForSeconds(tester, 5);

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // Кнопка logout: текст l10n.logout. На разных экранах layout варьируется,
      // поэтому ищем по тексту + проверяем, что нашли что-то tappable.
      await expectEventually(
        tester,
        find.text(l10n.logout),
        timeout: const Duration(seconds: 10),
        reason: 'logout button on profile screen',
      );
      await tester.tap(find.text(l10n.logout).first);
      await tester.pumpAndSettle(const Duration(seconds: 1));

      // Confirm-диалог (если есть) — нажимаем «Выйти». Иначе сразу logout.
      // Используем «soft» find: если диалог не показался, шаг пропускается.
      final confirmFinder = find.widgetWithText(FilledButton, l10n.logout);
      if (confirmFinder.evaluate().isNotEmpty) {
        await tester.tap(confirmFinder.first);
        await tester.pumpAndSettle(const Duration(seconds: 1));
      }

      // POST /auth/logout → clear tokens → state=null → authGuard → /.
      await pumpForSeconds(tester, 8);

      final path = GoRouter.of(
        anyScaffoldContext(tester),
      ).routerDelegate.currentConfiguration.uri.path;
      // Допускаем landing / login: оба значат «вышли».
      expect(
        path == '/' || path == '/login',
        isTrue,
        reason: 'after logout user is on landing or login (was: $path)',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
