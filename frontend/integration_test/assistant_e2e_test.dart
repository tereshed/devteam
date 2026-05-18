// Sprint 21 §Verification — E2E integration-тест Assistant Sidebar.
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
// Запуск:
//   make e2e-frontend
//   # или вручную:
//   docker compose up -d
//   cd frontend && flutter test integration_test/assistant_e2e_test.dart -d macos

import 'dart:convert';
import 'dart:io';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/app_router.dart';
import 'package:frontend/core/storage/token_provider.dart';
import 'package:frontend/core/theme/app_theme.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_message_bubble.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tool_call_card.dart';
import 'package:frontend/features/auth/domain/models/user_model.dart';
import 'package:frontend/features/auth/presentation/controllers/auth_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

const _apiBase = 'http://127.0.0.1:8080';

Future<bool> _backendAvailable() async {
  final client = HttpClient();
  client.connectionTimeout = const Duration(seconds: 2);
  try {
    final req = await client.getUrl(Uri.parse('$_apiBase/health'));
    final resp = await req.close().timeout(const Duration(seconds: 3));
    await resp.drain<void>();
    return resp.statusCode == 200;
  } catch (_) {
    return false;
  } finally {
    client.close(force: true);
  }
}

class _SeedCreds {
  _SeedCreds(this.token, this.user);
  final String token;
  final UserModel user;
}

/// Возвращает id самой свежей активной сессии ассистента seed-юзера —
/// тот, который контроллер фронта поднял через `ensureSession` (см.
/// `assistant_chat_controller.dart`).
Future<String> _sessionIdFromHistory(String token) async {
  final client = HttpClient();
  try {
    final req = await client.getUrl(
        Uri.parse('$_apiBase/api/v1/assistant/sessions?limit=1'));
    req.headers.set('Authorization', 'Bearer $token');
    final resp = await req.close();
    if (resp.statusCode != 200) {
      // HttpClientResponse — это Stream; неосушённый стрим оставит сокет
      // висеть и client.close() будет блокировать teardown теста до таймаута.
      await resp.drain<void>();
      throw Exception('list sessions failed: ${resp.statusCode}');
    }
    final json = jsonDecode(await resp.transform(utf8.decoder).join())
        as Map<String, dynamic>;
    final sessions = (json['sessions'] as List).cast<Map<String, dynamic>>();
    if (sessions.isEmpty) {
      throw Exception('no assistant sessions found for seed user');
    }
    return sessions.first['id'] as String;
  } finally {
    // force:true гарантирует, что зависшие/упавшие соединения не задержат
    // teardown интеграционного теста.
    client.close(force: true);
  }
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
  final client = HttpClient();
  try {
    while (DateTime.now().isBefore(deadline)) {
      final req = await client.getUrl(Uri.parse(
        '$_apiBase/api/v1/assistant/sessions/$sessionId/messages?limit=50',
      ));
      req.headers.set('Authorization', 'Bearer $token');
      final resp = await req.close();
      if (resp.statusCode == 200) {
        final body = await resp.transform(utf8.decoder).join();
        final json = jsonDecode(body) as Map<String, dynamic>;
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
      } else {
        // Стрим, который мы не парсим, нужно явно осушить — иначе соединение
        // не освободится, и client.close() заблокирует teardown теста.
        await resp.drain<void>();
      }
      await Future<void>.delayed(const Duration(milliseconds: 500));
    }
    return null;
  } finally {
    client.close(force: true);
  }
}

/// Создаёт пустой local-проект для seed-юзера. Нужен, чтобы у ассистента
/// был реалистичный контент при tool-loop (project_list возвращает не пустоту
/// — иначе LLM может зациклиться на «нет проектов, что делать?»).
Future<String> _createLocalProject(String token) async {
  final client = HttpClient();
  try {
    final r = Random.secure();
    final name = 'assistant-e2e-${List.generate(6, (_) => r.nextInt(16).toRadixString(16)).join()}';
    final body = jsonEncode({
      'name': name,
      'description': 'auto-created for assistant integration test',
      'git_provider': 'local',
    });
    final req = await client.postUrl(Uri.parse('$_apiBase/api/v1/projects'));
    req.headers.set('Content-Type', 'application/json');
    req.headers.set('Authorization', 'Bearer $token');
    req.write(body);
    final resp = await req.close();
    if (resp.statusCode != 201) {
      // Тело пригождается для error-message — вычитываем целиком (это тоже
      // дренаж стрима, отдельный drain() не нужен).
      final text = await resp.transform(utf8.decoder).join();
      throw Exception('create project failed: ${resp.statusCode} $text');
    }
    final json = jsonDecode(await resp.transform(utf8.decoder).join())
        as Map<String, dynamic>;
    return json['id'] as String;
  } finally {
    client.close(force: true);
  }
}

Future<_SeedCreds> _registerAndSeed() async {
  final r = Random.secure();
  final id = List.generate(8, (_) => r.nextInt(16).toRadixString(16)).join();
  final email = 'flutter-assistant-e2e-$id@example.com';
  final body = jsonEncode({'email': email, 'password': 'Password123!'});
  final client = HttpClient();
  try {
    final req =
        await client.postUrl(Uri.parse('$_apiBase/api/v1/auth/register'));
    req.headers.set('Content-Type', 'application/json');
    req.write(body);
    final resp = await req.close();
    if (resp.statusCode != 201) {
      await resp.drain<void>();
      throw Exception('register failed: ${resp.statusCode}');
    }
    final json = jsonDecode(await resp.transform(utf8.decoder).join())
        as Map<String, dynamic>;
    final token = json['access_token'] as String;

    final meReq = await client.getUrl(Uri.parse('$_apiBase/api/v1/auth/me'));
    meReq.headers.set('Authorization', 'Bearer $token');
    final meResp = await meReq.close();
    if (meResp.statusCode != 200) {
      await meResp.drain<void>();
      throw Exception('/me failed: ${meResp.statusCode}');
    }
    final meJson = jsonDecode(await meResp.transform(utf8.decoder).join())
        as Map<String, dynamic>;
    return _SeedCreds(token, UserModel.fromJson(meJson));
  } finally {
    client.close(force: true);
  }
}

Future<void> _expectEventually(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = const Duration(seconds: 10),
  String? reason,
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    if (finder.evaluate().isNotEmpty) {
      return;
    }
  }
  fail('Timeout waiting for ${reason ?? finder.toString()}');
}

/// `true`, как только finder ВПЕРВЫЕ что-то нашёл; иначе `false` после
/// таймаута. Не валит тест — используется для best-effort ассертов
/// (ответ ассистента, который зависит от наличия LLM-кредов в окружении).
Future<bool> _eventually(
  WidgetTester tester,
  Finder finder, {
  Duration timeout = const Duration(seconds: 10),
}) async {
  final deadline = DateTime.now().add(timeout);
  while (DateTime.now().isBefore(deadline)) {
    await tester.pump(const Duration(milliseconds: 200));
    if (finder.evaluate().isNotEmpty) {
      return true;
    }
  }
  return false;
}

BuildContext _anyContext(WidgetTester tester) =>
    tester.element(find.byType(Scaffold).first);

class _TestApp extends StatelessWidget {
  const _TestApp();
  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      onGenerateTitle: (context) => AppLocalizations.of(context)!.appTitle,
      theme: AppTheme.lightTheme,
      darkTheme: AppTheme.darkTheme,
      themeMode: ThemeMode.light,
      routerConfig: AppRouter.router,
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      supportedLocales: const [Locale('ru', ''), Locale('en', '')],
    );
  }
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'assistant sidebar: register → sidebar visible → send message → user bubble',
    (tester) async {
      if (!await _backendAvailable()) {
        markTestSkipped(
            'backend at $_apiBase is not reachable; start with `docker compose up -d` and re-run');
        return;
      }

      // 1) Свежий юзер на каждом запуске → чистый assistant-state. Дополнительно
      // создаём local-проект — даёт ассистенту реалистичный контент при tool-loop
      // (например, project_list возвращает 1 запись, а не пустой массив, что
      // иначе может затянуть LLM в "проектов нет — запрашиваю инструкции").
      final creds = await _registerAndSeed();
      await _createLocalProject(creds.token);

      // Desktop-размер обязателен: на mobile/tablet AppShell держит assistant
      // в endDrawer, а не как inline-колонку (см. `app_shell.dart` §responsive).
      await tester.binding.setSurfaceSize(const Size(1600, 900));

      // 2) Поднимаем приложение с предзаданными токеном + пользователем.
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            accessTokenProvider
                .overrideWith(() => _SeededAccessToken(creds.token)),
            authControllerProvider
                .overrideWith(() => _SeededAuthController(creds.user)),
          ],
          child: const _TestApp(),
        ),
      );
      await tester.pumpAndSettle(const Duration(seconds: 2));

      // 3) Стартовый экран — /projects (AppShell с AssistantSidebar).
      // Не заходим внутрь проекта намеренно: чат-screen вне AppShell, и сайдбар
      // там не виден. WS из integration_test поднять надёжно не получилось
      // (см. soft-pass-ветку ниже), поэтому факт прихода LLM-ответа проверяем
      // напрямую через REST `/messages` — он отдаёт всё, что записал агент-луп.
      GoRouter.of(_anyContext(tester)).go('/projects');
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 250));
      }

      final l10n = AppLocalizations.of(_anyContext(tester))!;

      // 4) AssistantSidebar по умолчанию open=true на desktop, но реальный
      // размер окна macOS-integration test runner мы не контролируем —
      // setSurfaceSize в integration_test игнорируется реальным window manager.
      // Поэтому если sidebar НЕ виден сразу — тапаем toggle в AppBar:
      //   • desktop closed   → toggleOpen() → inline-колонка появляется.
      //   • tablet / mobile  → openEndDrawer() → AssistantSidebar в endDrawer.
      final titleFinder = find.text(l10n.assistantSidebarTitle);
      final alreadyVisible = await _eventually(
        tester,
        titleFinder,
        timeout: const Duration(seconds: 5),
      );
      if (!alreadyVisible) {
        final toggle = find.byTooltip(l10n.assistantToggleTooltip);
        expect(toggle, findsOneWidget,
            reason: 'AppBar assistant toggle button');
        await tester.tap(toggle);
        await tester.pumpAndSettle(const Duration(seconds: 1));
      }
      await _expectEventually(
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
      await _expectEventually(
        tester,
        find.text(l10n.assistantEmptyChat),
        timeout: const Duration(seconds: 15),
        reason: 'empty-chat hint visible after session bootstrap',
      );

      // 6) Вводим сообщение и отправляем через UI.
      final input = find.byKey(const ValueKey('chat_input_field'));
      expect(input, findsOneWidget,
          reason: 'chat input field visible inside assistant sidebar');
      const userText = 'integration-test ping';
      await tester.enterText(input, userText);
      await tester.pump();

      final sendBtn = find.byKey(const ValueKey('chat_send_button'));
      expect(sendBtn, findsOneWidget);
      await tester.tap(sendBtn);
      // POST /messages → 202; user-bubble появляется как ответ репозитория.
      for (var i = 0; i < 20; i++) {
        await tester.pump(const Duration(milliseconds: 250));
      }

      // 7) User-bubble с введённым текстом виден.
      await _expectEventually(
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
      expect(reply, isNotNull,
          reason: 'agent-loop must produce a final assistant message; check '
              'docker logs wibe_backend for `assistant: loop failed` warnings');
      // ignore: avoid_print
      print('[assistant-e2e] backend reply via REST:\n${reply!.substring(0, reply.length.clamp(0, 200))}'
          '${reply.length > 200 ? "…" : ""}');

      final assistantBubble = find.byWidgetPredicate(
        (w) =>
            (w is AssistantMessageBubble &&
                w.message.role != assistantMessageRoleUser) ||
            w is AssistantToolCallCard,
      );
      final uiSawBubble = await _eventually(
        tester,
        assistantBubble,
        timeout: const Duration(seconds: 5),
      );
      // ignore: avoid_print
      print(uiSawBubble
          ? '[assistant-e2e] UI bubble rendered too — WS path also verified'
          : '[assistant-e2e] UI bubble not visible (WS likely not connected '
              'in integration_test env); REST end-to-end already proves the '
              'agent-loop works');
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}

class _SeededAccessToken extends AccessToken {
  _SeededAccessToken(this._seed);
  final String _seed;

  @override
  String? build() => _seed;

  @override
  Future<void> init() async => state = _seed;

  @override
  Future<void> setToken(String token) async => state = token;

  @override
  Future<void> clear() async => state = null;
}

class _SeededAuthController extends AuthController {
  _SeededAuthController(this._user);
  final UserModel _user;

  @override
  Future<UserModel?> build() async => _user;
}
