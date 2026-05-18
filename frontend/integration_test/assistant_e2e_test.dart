// Sprint 21 §Verification — E2E integration-тест Assistant Sidebar.
//
// Sprint 23 (Phase 3) — переехал на `test_support/`: общие seed_creds /
// fresh_test_app / eventually-хелперы.
//
// Что проверяем:
//   1. Регистрация пользователя через REST (минуя UI-логин — keychain недоступен
//      на macOS test-runner без entitlements; токен инжектится через override).
//   2. AppShell на desktop держит сайдбар открытым (open=true по умолчанию):
//      рендерится заголовок `Assistant` и tab «Chat».
//   3. После bootstrap'a ChatController создаёт/выбирает сессию и тянет историю
//      → "empty chat" hint виден.
//   4. Ввод текста в `chat_input_field` + tap `chat_send_button`:
//      • repository.sendMessage отрабатывает на реальном бэке (POST 202);
//      • в UI появляется user-bubble с введённым текстом.
//   5. (best-effort) Ждём, не прилетит ли ответ ассистента или системное
//      сообщение об ошибке (зависит от LLM-конфига). Этот шаг не валит тест,
//      если ответа за таймаут не дождались — главный assert уже выше.
//
// **ВАЖНО про cost-leak:** этот тест единственный в integration_test/, который
// действительно дёргает LLM (chat → agent-loop). Phase 3 Go-обёртка считает
// `SELECT COUNT(*) FROM llm_logs` ДО прогона и фиксирует прирост; если рост
// произошёл за пределами этого теста — это регрессия (см.
// `backend/test/featuresmoke/frontend_e2e_test.go`).
//
// Запуск:
//   make test-features-frontend
//   # или вручную:
//   docker compose up -d
//   cd frontend && flutter test integration_test/assistant_e2e_test.dart -d macos

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_message_bubble.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tool_call_card.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/rest_client.dart';
import 'test_support/seed_creds.dart';
import 'test_support/test_app.dart';

/// Возвращает id самой свежей активной сессии ассистента seed-юзера —
/// тот, который контроллер фронта поднял через `ensureSession` (см.
/// `assistant_chat_controller.dart`).
Future<String> _sessionIdFromHistory(String token) async {
  final json = await TestRestClient.get(
    '/assistant/sessions?limit=1',
    token: token,
  );
  final sessions = (json['sessions'] as List).cast<Map<String, dynamic>>();
  if (sessions.isEmpty) {
    throw StateError('no assistant sessions found for seed user');
  }
  return sessions.first['id'] as String;
}

/// Polling REST `/messages` пока не найдём финальное `role=assistant` сообщение
/// с непустым content (=== реальный LLM-ответ, не tool_call). Это надёжная
/// end-to-end проверка: бэк → agent-loop → LLM → запись в БД. WS-доставку до
/// UI integration_test не покрывает (см. comments в основном тесте).
///
/// Возвращает текст ответа ИЛИ null, если за `timeout` ничего не пришло
/// (LLM-провайдер не настроен / уперлись в rate-limit / иной soft-fail).
Future<String?> _waitForAssistantReplyViaRest({
  required String token,
  required String sessionId,
  Duration timeout = const Duration(seconds: 45),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    try {
      final json = await TestRestClient.get(
        '/assistant/sessions/$sessionId/messages?limit=50',
        token: token,
      );
      final messages = (json['messages'] as List).cast<Map<String, dynamic>>();
      for (final m in messages) {
        final role = m['role'] as String?;
        final content = m['content'] as String?;
        final toolCallId = m['tool_call_id'] as String?;
        // Финальный ответ — role=assistant, есть content и НЕТ tool_call_id
        // (assistant-сообщения с tool_call_id — это промежуточные tool-вызовы).
        if (role == 'assistant' &&
            content != null &&
            content.isNotEmpty &&
            (toolCallId == null || toolCallId.isEmpty)) {
          return content;
        }
      }
    } on RestRequestException {
      // Серверная икота (503 во время рестарта, 5xx) — это soft-сигнал в
      // long-poll'е, продолжаем тыкаться до deadline.
    }
    await Future<void>.delayed(const Duration(milliseconds: 500));
  }
  return null;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'assistant sidebar: register → sidebar visible → send message → user bubble',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // 1) Свежий юзер на каждом запуске → чистый assistant-state. Дополнительно
      // создаём local-проект — даёт ассистенту реалистичный контент при tool-loop
      // (например, project_list возвращает 1 запись, а не пустой массив, что
      // иначе может затянуть LLM в "проектов нет — запрашиваю инструкции").
      final creds = await pumpFreshAuthedApp(
        tester,
        prefix: 'flutter-assistant-e2e',
      );
      await createLocalProject(creds.token, namePrefix: 'assistant-e2e');

      // Desktop-размер обязателен: на mobile/tablet AppShell держит assistant
      // в endDrawer, а не как inline-колонку (см. `app_shell.dart` §responsive).
      await tester.binding.setSurfaceSize(const Size(1600, 900));

      // 3) Стартовый экран — /projects (AppShell с AssistantSidebar).
      // Не заходим внутрь проекта намеренно: чат-screen вне AppShell, и сайдбар
      // там не виден. WS из integration_test поднять надёжно не получилось
      // (см. soft-pass-ветку ниже), поэтому факт прихода LLM-ответа проверяем
      // напрямую через REST `/messages` — он отдаёт всё, что записал агент-луп.
      GoRouter.of(anyScaffoldContext(tester)).go('/projects');
      await pumpForSeconds(tester, 5);

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // 4) AssistantSidebar по умолчанию open=true на desktop, но реальный
      // размер окна macOS-integration test runner мы не контролируем —
      // setSurfaceSize в integration_test игнорируется реальным window manager.
      // Поэтому если sidebar НЕ виден сразу — тапаем toggle в AppBar:
      //   • desktop closed   → toggleOpen() → inline-колонка появляется.
      //   • tablet / mobile  → openEndDrawer() → AssistantSidebar в endDrawer.
      final titleFinder = find.text(l10n.assistantSidebarTitle);
      final alreadyVisible = await waitUntil(
        tester,
        titleFinder,
        timeout: const Duration(seconds: 5),
      );
      if (!alreadyVisible) {
        final toggle = find.byTooltip(l10n.assistantToggleTooltip);
        expect(
          toggle,
          findsOneWidget,
          reason: 'AppBar assistant toggle button',
        );
        await tester.tap(toggle);
        await tester.pumpAndSettle(const Duration(seconds: 1));
      }
      await expectEventually(
        tester,
        titleFinder,
        timeout: const Duration(seconds: 15),
        reason: 'assistant sidebar title visible',
      );

      // Вкладки Chat/Tasks отмечены ValueKey'ами — не зависят от локали.
      expect(find.byKey(const ValueKey('assistant_tab_chat')), findsOneWidget);
      expect(find.byKey(const ValueKey('assistant_tab_tasks')), findsOneWidget);

      // 5) После bootstrap ChatController создаёт/находит сессию и тянет
      // пустую историю → empty hint виден.
      await expectEventually(
        tester,
        find.text(l10n.assistantEmptyChat),
        timeout: const Duration(seconds: 15),
        reason: 'empty-chat hint visible after session bootstrap',
      );

      // 6) Вводим сообщение и отправляем через UI.
      final input = find.byKey(const ValueKey('chat_input_field'));
      expect(
        input,
        findsOneWidget,
        reason: 'chat input field visible inside assistant sidebar',
      );
      const userText = 'integration-test ping';
      await tester.enterText(input, userText);
      await tester.pump();

      final sendBtn = find.byKey(const ValueKey('chat_send_button'));
      expect(sendBtn, findsOneWidget);
      await tester.tap(sendBtn);
      // POST /messages → 202; user-bubble появляется как ответ репозитория.
      await pumpForSeconds(tester, 5);

      // 7) User-bubble с введённым текстом виден.
      await expectEventually(
        tester,
        find.text(userText),
        timeout: const Duration(seconds: 10),
        reason: 'user bubble with sent text',
      );

      // 8) End-to-end проверка ответа ассистента — через REST, а не UI.
      //
      // Почему не UI: WebSocket поднимается только при заходе в /projects/{id}/chat
      // (см. chat_controller.dart), а сайдбар живёт в /projects (AppShell вне
      // project-dashboard). Сшить эти два сценария в одном integration_test
      // надёжно не получается, поэтому факт прихода LLM-ответа проверяем
      // напрямую REST'ом — он отражает то, что бэкенд записал в БД после
      // полного agent-loop (system → tool_use → tool_result → final text).
      //
      // Это закрывает регрессы:
      //   • wiring AssistantHandler в server.go
      //   • Anthropic tool-schema (oneOf/anyOf на верхнем уровне → 400)
      //   • Anthropic tool-result message format (role:"tool" → 400)
      //
      // Дополнительно — best-effort assert на UI-bubble: вдруг WS живой.
      final sessionId = await _sessionIdFromHistory(creds.token);
      final reply = await _waitForAssistantReplyViaRest(
        token: creds.token,
        sessionId: sessionId,
      );
      expect(
        reply,
        isNotNull,
        reason:
            'agent-loop must produce a final assistant message; check '
            'docker logs wibe_backend for `assistant: loop failed` warnings',
      );
      // ignore: avoid_print
      print(
        '[assistant-e2e] backend reply via REST:\n${reply!.substring(0, reply.length.clamp(0, 200))}'
        '${reply.length > 200 ? "…" : ""}',
      );

      final assistantBubble = find.byWidgetPredicate(
        (w) =>
            (w is AssistantMessageBubble &&
                w.message.role != assistantMessageRoleUser) ||
            w is AssistantToolCallCard,
      );
      final uiSawBubble = await waitUntil(
        tester,
        assistantBubble,
        timeout: const Duration(seconds: 5),
      );
      // ignore: avoid_print
      print(
        uiSawBubble
            ? '[assistant-e2e] UI bubble rendered too — WS path also verified'
            : '[assistant-e2e] UI bubble not visible (WS likely not connected '
                  'in integration_test env); REST end-to-end already proves the '
                  'agent-loop works',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
