// Sprint 14.2 — E2E integration-тест полного UI-flow:
// register (через REST) → авторизованная UI-сессия → /projects → создание
// проекта через форму → новый проект виден в списке.
//
// Авторизация через UI на macOS-test runner упирается в flutter_secure_storage
// + Keychain entitlements (без dev-сертификата FlutterSecureStorage не работает),
// поэтому шаги register/login делаем напрямую через REST, токен инжектим в
// accessTokenProvider при старте приложения через ProviderScope.overrides.
// Это покрывает то, что 14.2 действительно проверяет — полный UI-flow поверх
// реального бэкенда: переходы экранов, формы, отображение данных из API.
//
// Sprint 23 (Phase 3) — переехал на `test_support/`: общий `freshTestApp`,
// очистка хранилищ, единая фабрика SeededAccessToken/SeededAuthController.
//
// Запуск: при поднятом стеке `make test-features-frontend` (см. Makefile).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'full UI flow: create project via form → visible in list',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      await pumpFreshAuthedApp(tester, prefix: 'flutter-e2e');

      // Переходим на /projects (auth уже есть).
      GoRouter.of(anyScaffoldContext(tester)).go('/projects');
      // Дай времени API подгрузиться (/projects?limit=...).
      await pumpForSeconds(tester, 8);

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;
      try {
        await expectEventually(
          tester,
          find.text(l10n.projectsTitle),
          reason: 'projects list title',
        );
      } catch (e) {
        // ignore: avoid_print
        print('---- visible text on screen ----');
        for (final w in find.byType(Text).evaluate()) {
          final t = (w.widget as Text).data;
          if (t != null) {
            // ignore: avoid_print
            print('  $t');
          }
        }
        // ignore: avoid_print
        print(
          '---- current location: ${GoRouter.of(anyScaffoldContext(tester)).routerDelegate.currentConfiguration.uri} ----',
        );
        rethrow;
      }

      // ── Шаг A: открываем форму создания проекта.
      final createBtn = find.text(l10n.createProject);
      await expectEventually(
        tester,
        createBtn,
        reason: 'create project button',
      );
      await tester.tap(createBtn.first);
      await tester.pumpAndSettle(const Duration(seconds: 2));
      await expectEventually(
        tester,
        find.text(l10n.createProjectScreenTitle),
        reason: 'create project screen',
      );

      // ── Шаг B: заполняем форму. Дефолтный gitProvider в UI = "github" и
      // требует валидного git_credential на бэкенде. В тесте — переключаемся на
      // "local" (без удалённого URL): ProjectService разрешает local-провайдер
      // без git_url и без credential.
      final projectName =
          'Flutter E2E ${DateTime.now().millisecondsSinceEpoch}';
      final cpFields = find.byType(TextFormField);
      expect(
        cpFields,
        findsAtLeast(2),
        reason: 'create project screen has at least name + description',
      );
      await tester.enterText(cpFields.at(0), projectName);
      await tester.enterText(
        cpFields.at(1),
        'Created from Flutter integration test',
      );

      // Открываем dropdown и выбираем «Local».
      await tester.tap(find.byType(DropdownButtonFormField<String>));
      await tester.pumpAndSettle(const Duration(seconds: 1));
      // У DropdownMenuItem текст «Local» — берём текст из l10n.
      final localOption = find.text(l10n.gitProviderLocal);
      expect(
        localOption,
        findsAtLeastNWidgets(1),
        reason: 'local provider option in dropdown',
      );
      await tester.tap(localOption.last);
      await tester.pumpAndSettle(const Duration(seconds: 1));

      final createSubmit = find.widgetWithText(FilledButton, l10n.create);
      expect(createSubmit, findsOneWidget);
      await tester.tap(createSubmit);
      // Сетевой запрос → возврат на список → обновление; pumpAndSettle не ждёт IO.
      await pumpForSeconds(tester, 10);

      // ── Шаг C: после Create UI редиректит на /projects/{id}/chat
      // (project dashboard). Возвращаемся на список и проверяем, что
      // новый проект в нём виден.
      GoRouter.of(anyScaffoldContext(tester)).go('/projects');
      await pumpForSeconds(tester, 8);
      await expectEventually(
        tester,
        find.text(l10n.projectsTitle),
        timeout: const Duration(seconds: 10),
        reason: 'back to projects list',
      );
      await expectEventually(
        tester,
        find.text(projectName),
        timeout: const Duration(seconds: 10),
        reason: 'newly created project visible in list',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
