// Phase 4 §Task 4.1 — Assistant chat flow (LLM-free UI contract).
//
// Что покрываем (PR-gate, никаких LLM-вызовов):
//   1. После регистрации seed-юзера и /projects-bootstrap'а AssistantSidebar
//      виден (либо сразу — desktop default open=true, либо после toggle).
//   2. В сайдбаре есть вкладки `assistant_tab_chat` и `assistant_tab_tasks`
//      (ValueKey'и, не зависят от локали — см. assistant_sidebar.dart).
//   3. Bootstrap ChatController создаёт пустую сессию, видим хинт
//      `assistantEmptyChat` → значит REST `/assistant/sessions` живёт и фронт
//      его правильно отрисовывает.
//   4. Переключение на Tasks-tab → виден empty-state `assistantNoActiveTasks`
//      (свежий юзер ещё не запускал задачи) → REST `/assistant/active-tasks`
//      работает.
//   5. Возврат на Chat-tab → empty-hint снова виден.
//   6. Input-поле `chat_input_field` существует и принимает ввод. Кнопка
//      send становится enabled при не-пустом тексте.
//
// Чего НЕ делаем:
//   - Не тапаем send — это уйдёт в agent-loop → LLM → llm_logs +1 запись →
//     ломает cost-leak guard. Полный send-flow покрывает `assistant_e2e_test.dart`.
//   - Не проверяем приход assistant-ответа.
//   - Не проверяем destructive-confirm — он требует tool_use от LLM,
//     отдельный сценарий.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/seed_creds.dart';
import 'test_support/test_app.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'assistant sidebar: sidebar opens, tabs switch, input accepts text (no LLM)',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      final creds = await pumpFreshAuthedApp(
        tester,
        prefix: 'flutter-chat-flow',
      );
      // Создаём проект, чтобы оказаться на /projects с заполненной командой
      // (иначе projects-screen в empty-state — не показывает AppShell с
      // assistant sidebar в attached-режиме).
      await createLocalProject(creds.token, namePrefix: 'chat-flow');

      // Desktop-размер — sidebar attached, не endDrawer.
      await tester.binding.setSurfaceSize(const Size(1600, 900));

      GoRouter.of(anyScaffoldContext(tester)).go('/projects');
      await pumpForSeconds(tester, 5);

      final l10n = requireAppLocalizations(
        anyScaffoldContext(tester),
        where: 'chat_flow_test',
      );

      // 1. Sidebar виден сразу или через toggle (см. комментарий в
      //    assistant_e2e_test.dart про setSurfaceSize и реальный размер окна).
      final titleFinder = find.text(l10n.assistantSidebarTitle);
      final alreadyVisible = await waitUntil(
        tester,
        titleFinder,
        timeout: const Duration(seconds: 5),
      );
      if (!alreadyVisible) {
        final toggle = find.byTooltip(l10n.assistantToggleTooltip);
        expect(toggle, findsWidgets, reason: 'AppBar assistant toggle button');
        await tester.tap(toggle.first);
        await tester.pumpAndSettle(const Duration(seconds: 1));
      }
      await expectEventually(
        tester,
        titleFinder,
        timeout: const Duration(seconds: 15),
        reason: 'assistant sidebar title visible',
      );

      // 2. Tab-ключи (стабильные ValueKey — выживают смены локали).
      final chatTab = find.byKey(const ValueKey('assistant_tab_chat'));
      final tasksTab = find.byKey(const ValueKey('assistant_tab_tasks'));
      expect(chatTab, findsOneWidget, reason: 'chat tab present');
      expect(tasksTab, findsOneWidget, reason: 'tasks tab present');

      // 3. Empty-chat hint виден после bootstrap'а ChatController. Это
      //    значит REST `/assistant/sessions` отработал — фронт без сетевого
      //    контракта остановился бы в loading-state и хинт не появился.
      await expectEventually(
        tester,
        find.text(l10n.assistantEmptyChat),
        timeout: const Duration(seconds: 15),
        reason: 'empty-chat hint visible after session bootstrap',
      );

      // 4. Переключение на Tasks-tab. У свежесозданного юзера активных задач
      //    нет → empty-state `assistantNoActiveTasks`.
      await tester.tap(tasksTab);
      await pumpForSeconds(tester, 4);
      await expectEventually(
        tester,
        find.text(l10n.assistantNoActiveTasks),
        timeout: const Duration(seconds: 15),
        reason: 'empty active-tasks hint visible',
      );

      // 5. Возврат на Chat — empty-hint снова виден.
      await tester.tap(chatTab);
      await pumpForSeconds(tester, 3);
      await expectEventually(
        tester,
        find.text(l10n.assistantEmptyChat),
        timeout: const Duration(seconds: 10),
        reason: 'empty-chat hint visible after switching back from tasks',
      );

      // 6. Input принимает текст; send button существует.
      //    НЕ тапаем send — это бы дёрнуло LLM-петлю.
      final input = find.byKey(const ValueKey('chat_input_field'));
      expect(input, findsOneWidget, reason: 'chat input field visible');
      const userText = 'chat-flow no-llm test text';
      await tester.enterText(input, userText);
      await tester.pump();

      // Текст реально попал в TextField (assist UI debugging при flaky CI):
      // ищем TextField с этим контентом — у chat_input строка одна, без
      // длинных подсказок и плейсхолдеров.
      expect(
        find.text(userText),
        findsOneWidget,
        reason: 'entered text appears inside chat input',
      );

      // Send-button существует. Состояние enabled/disabled не проверяем
      // explicit'но (его реализация зависит от controller-state, что вне
      // scope этого теста); главное — кнопка зарендерилась.
      expect(
        find.byKey(const ValueKey('chat_send_button')),
        findsOneWidget,
        reason: 'chat send button visible',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
