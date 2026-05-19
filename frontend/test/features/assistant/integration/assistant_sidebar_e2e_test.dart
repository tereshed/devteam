// Sprint 21 §Verification — frontend e2e widget-flow тест для Assistant Sidebar.
//
// Это «end-to-end на уровне UI»: реальный Riverpod, реальные провайдеры
// (`AssistantChatController`, `AssistantTasksController`, `AssistantSidebar`),
// реальная локализация — но границы процесса (HTTP/WS) мокаются:
//
//   • `assistantRepositoryProvider` → mockito-stub поверх `AssistantRepository`.
//   • `webSocketServiceProvider`    → mockito-stub поверх `WebSocketService`,
//     `events` возвращает наш `StreamController<WsClientEvent>`, через который
//     тест эмитит assistant.* события так же, как бы их слал бэкенд.
//
// Контракт устойчивости:
//   - Все надписи UI достаются через `requireAppLocalizations` (docs/rules/frontend.md
//     §2.3, review.md «Хардкод строк»). Тест не зависит от текстов `.arb`.
//   - Вкладки и кнопки Approve/Deny ищутся по `ValueKey` — нулевая зависимость
//     от локали.
//
// Сценарии (см. план §Verification «Frontend»):
//   1. Открыть сайдбар → empty hint виден → отправить сообщение → user-bubble →
//      WS assistant.message → assistant-bubble.
//   2. WS tool_call + tool_result → ToolCallCard рендерит имя tool'а + status badge.
//   3. WS confirm_request → inline-карточка с Approve/Deny → tap → repository.confirmToolCall.
//   4. Tasks-tab: REST-список + WS task_update.
//   5. session_updated busy=true → busy-индикатор виден.
//
// Запуск:
//   cd frontend && flutter test test/features/assistant/integration/

// @dart=2.19
@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/data/assistant_repository.dart';
import 'package:frontend/features/assistant/domain/assistant_active_task_model.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/domain/assistant_status_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_sidebar.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import 'assistant_sidebar_e2e_test.mocks.dart';

// Mocks: AssistantRepository + WebSocketService (build_runner сгенерит
// assistant_sidebar_e2e_test.mocks.dart).
@GenerateNiceMocks([
  MockSpec<AssistantRepository>(),
  MockSpec<WebSocketService>(),
])
void main() {
  const userId = '11111111-1111-1111-1111-111111111111';
  const sessionId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';

  late MockAssistantRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;

  AssistantSessionModel session({bool busy = false}) => AssistantSessionModel(
        id: sessionId,
        userId: userId,
        status: 'active',
        busy: busy,
        createdAt: DateTime.utc(2026, 5, 17),
        updatedAt: DateTime.utc(2026, 5, 17),
      );

  AssistantMessageModel msg({
    required String id,
    required String role,
    String? content,
    String? toolCallId,
    String? toolName,
    Map<String, dynamic>? toolArguments,
    Map<String, dynamic>? toolResult,
    DateTime? createdAt,
  }) {
    return AssistantMessageModel(
      id: id,
      sessionId: sessionId,
      role: role,
      content: content,
      toolCallId: toolCallId,
      toolName: toolName,
      toolArguments: toolArguments,
      toolResult: toolResult,
      createdAt: createdAt ?? DateTime.utc(2026, 5, 17, 12, 0),
    );
  }

  void stubBootstrap({List<AssistantMessageModel> history = const []}) {
    when(mockRepo.getStatus(cancelToken: anyNamed('cancelToken')))
        .thenAnswer((_) async => const AssistantStatusModel(
              isConfigured: true,
              requiredProvider: 'anthropic',
            ));
    when(mockRepo.listSessions(
      limit: anyNamed('limit'),
      includeArchived: anyNamed('includeArchived'),
      cancelToken: anyNamed('cancelToken'),
    )).thenAnswer((_) async => AssistantSessionListResponse(
          sessions: [session()],
        ));
    when(mockRepo.getMessages(
      any,
      limit: anyNamed('limit'),
      beforeCreatedAt: anyNamed('beforeCreatedAt'),
      beforeId: anyNamed('beforeId'),
      cancelToken: anyNamed('cancelToken'),
    )).thenAnswer((_) async => AssistantMessageListResponse(
          messages: history,
          limit: 30,
          hasMore: false,
        ));
    when(mockRepo.getSession(any, cancelToken: anyNamed('cancelToken')))
        .thenAnswer((_) async => session());
  }

  setUp(() {
    mockRepo = MockAssistantRepository();
    mockWs = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(mockWs.events).thenAnswer((_) => wsEvents.stream);
  });

  tearDown(() async {
    await wsEvents.close();
  });

  Widget appUnderTest() {
    return ProviderScope(
      overrides: [
        assistantRepositoryProvider.overrideWithValue(mockRepo),
        webSocketServiceProvider.overrideWithValue(mockWs),
      ],
      child: MaterialApp(
        locale: const Locale('en'),
        supportedLocales: AppLocalizations.supportedLocales,
        localizationsDelegates: const [
          AppLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
        ],
        home: const Scaffold(
          // Шире, чем дефолт правой панели в AppShell (360dp), чтобы task-row
          // c длинными переводными подписями не уходил в overflow при тестах.
          body: SizedBox(width: 600, child: AssistantSidebar()),
        ),
      ),
    );
  }

  /// Достаёт `AppLocalizations` из дерева через тот же helper, что и продакшен-
  /// виджеты. Тест не зависит от текстов локализации (review.md «Хардкод строк»).
  AppLocalizations l10nOf(WidgetTester tester) {
    final ctx = tester.element(find.byType(Scaffold).first);
    return requireAppLocalizations(ctx, where: 'assistant_sidebar_e2e_test');
  }

  group('AssistantSidebar — e2e flow', () {
    testWidgets(
        'empty chat → send message → user bubble → WS assistant message → final bubble',
        (tester) async {
      stubBootstrap();
      when(mockRepo.sendMessage(
        any,
        content: anyNamed('content'),
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((inv) async {
        final content = inv.namedArguments[#content] as String;
        final clientMessageId = inv.namedArguments[#clientMessageId] as String;
        return SendAssistantMessageResponse(
          message: AssistantMessageModel(
            id: 'user-msg-1',
            sessionId: sessionId,
            role: 'user',
            content: content,
            clientMessageId: clientMessageId,
            createdAt: DateTime.utc(2026, 5, 17, 12, 0),
          ),
          duplicate: false,
        );
      });

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();

      final l10n = l10nOf(tester);

      // Header + tabs отрисовались (вкладки — через ValueKey, заголовок —
      // через l10n).
      expect(find.text(l10n.assistantSidebarTitle), findsOneWidget);
      expect(find.byKey(const ValueKey('assistant_tab_chat')), findsOneWidget);
      expect(find.byKey(const ValueKey('assistant_tab_tasks')), findsOneWidget);

      // Пустая чат-секция показала empty hint (история пуста).
      expect(find.text(l10n.assistantEmptyChat), findsOneWidget,
          reason: 'empty hint visible before any message');

      // Вводим сообщение и отправляем.
      final input = find.byKey(const ValueKey('chat_input_field'));
      expect(input, findsOneWidget);
      await tester.enterText(input, 'привет, ассистент');
      await tester.pump();

      final sendBtn = find.byKey(const ValueKey('chat_send_button'));
      expect(sendBtn, findsOneWidget);
      await tester.tap(sendBtn);
      await tester.pump();
      await tester.pumpAndSettle();

      // repository.sendMessage вызван ровно один раз с правильным контентом.
      verify(mockRepo.sendMessage(
        sessionId,
        content: 'привет, ассистент',
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);

      // Виджет рендерит user-bubble с нашим текстом.
      expect(find.text('привет, ассистент'), findsOneWidget);

      // Бэкенд через WS присылает ответ ассистента.
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantMessage(WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 0, 5),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          messageId: 'assistant-msg-1',
          role: 'assistant',
          content: 'Привет! Чем помочь?',
          createdAt: DateTime.utc(2026, 5, 17, 12, 0, 5),
        )),
      ));
      await tester.pumpAndSettle();

      expect(find.text('Привет! Чем помочь?'), findsOneWidget);
    });

    testWidgets(
        'WS tool_call + tool_result → ToolCallCard renders tool name and OK status badge',
        (tester) async {
      stubBootstrap();

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();
      final l10n = l10nOf(tester);

      // Эмулируем, что бэкенд решил вызвать project_list.
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantMessage(WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 1),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          messageId: 'tc-msg-1',
          role: 'assistant',
          toolCallId: 'call_1',
          toolName: 'project_list',
          createdAt: DateTime.utc(2026, 5, 17, 12, 1),
        )),
      ));
      // Pre-фиксируем tool-row (без toolResult — pending).
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantMessage(WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 1, 1),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          messageId: 'tr-msg-1',
          role: 'tool',
          toolCallId: 'call_1',
          toolName: 'project_list',
          createdAt: DateTime.utc(2026, 5, 17, 12, 1, 1),
        )),
      ));
      // Отдельный tool_result-event несёт status=ok + payload. Контроллер
      // мерджит его в существующую tool-row (см. _applyToolResultEvent).
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantToolResult(WsAssistantToolResultEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 1, 2),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'call_1',
          toolName: 'project_list',
          status: 'ok',
          result: const {'items': []},
        )),
      ));
      await tester.pumpAndSettle();

      // ToolCallCard рендерит имя tool'а (l10n.assistantToolCallTitle = "Tool {tool}").
      expect(find.text(l10n.assistantToolCallTitle('project_list')),
          findsOneWidget,
          reason: 'tool name renders via assistantToolCallTitle template');

      // Status badge "OK" (l10n.assistantToolResultStatusOk).
      expect(find.text(l10n.assistantToolResultStatusOk), findsOneWidget,
          reason: 'OK status badge after tool_result with status=ok');
    });

    testWidgets(
        'confirm_request → Approve tap (via key) → repository.confirmToolCall(approved:true)',
        (tester) async {
      stubBootstrap();
      when(mockRepo.confirmToolCall(
        any,
        toolCallId: anyNamed('toolCallId'),
        approved: anyNamed('approved'),
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => const ConfirmToolCallResponse(accepted: true));

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();
      final l10n = l10nOf(tester);

      // Бэкенд просит подтвердить destructive операцию.
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantConfirmRequest(WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 2),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'call_destructive',
          toolName: 'project_delete',
          arguments: const {'id': 'p1'},
          summary: 'Delete project p1?',
        )),
      ));
      await tester.pumpAndSettle();

      // Inline-confirm-карточка отрисовалась: title + summary + кнопки по
      // ValueKey'ам (без зависимости от локали).
      expect(find.text(l10n.assistantConfirmTitle), findsOneWidget);
      expect(find.text('Delete project p1?'), findsOneWidget);
      final approveKey = find.byKey(const ValueKey('assistant_confirm_approve'));
      final denyKey = find.byKey(const ValueKey('assistant_confirm_deny'));
      expect(approveKey, findsOneWidget);
      expect(denyKey, findsOneWidget);

      // Тап Approve (по ключу — тест не зависит от текста "Approve"/"Одобрить").
      await tester.tap(approveKey);
      await tester.pumpAndSettle();

      verify(mockRepo.confirmToolCall(
        sessionId,
        toolCallId: 'call_destructive',
        approved: true,
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    testWidgets(
        'confirm_request → Deny tap (via key) → repository.confirmToolCall(approved:false)',
        (tester) async {
      stubBootstrap();
      when(mockRepo.confirmToolCall(
        any,
        toolCallId: anyNamed('toolCallId'),
        approved: anyNamed('approved'),
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => const ConfirmToolCallResponse(accepted: true));

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();

      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantConfirmRequest(WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 3),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'call_dn',
          toolName: 'project_delete',
          arguments: const {'id': 'p1'},
        )),
      ));
      await tester.pumpAndSettle();

      await tester.tap(find.byKey(const ValueKey('assistant_confirm_deny')));
      await tester.pumpAndSettle();

      verify(mockRepo.confirmToolCall(
        sessionId,
        toolCallId: 'call_dn',
        approved: false,
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    testWidgets('Tasks tab → REST list rendered, WS task_update appends',
        (tester) async {
      stubBootstrap();
      when(mockRepo.getActiveTasks(cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => AssistantActiveTasksResponse(
                tasks: [
                  AssistantActiveTaskModel(
                    taskId: 'task-1',
                    projectId: 'proj-1',
                    projectName: 'Demo',
                    title: 'First task',
                    state: 'active',
                    updatedAt: DateTime.utc(2026, 5, 17, 11, 0),
                  ),
                ],
              ));

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();

      // Переключаемся на вкладку Tasks через ValueKey — независимо от локали.
      await tester.tap(find.byKey(const ValueKey('assistant_tab_tasks')));
      await tester.pumpAndSettle();

      expect(find.text('First task'), findsOneWidget,
          reason: 'REST-bootstrapped active task visible');

      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantTaskUpdate(WsAssistantTaskUpdateEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 5),
          v: 1,
          userId: userId,
          projectId: 'proj-1',
          taskId: 'task-2',
          state: 'active',
          title: 'Second task',
          updatedAt: DateTime.utc(2026, 5, 17, 12, 5),
        )),
      ));
      await tester.pumpAndSettle();

      expect(find.text('First task'), findsOneWidget);
      expect(find.text('Second task'), findsOneWidget);

      // Терминальное событие обновляет state, но карточка остаётся в списке
      // (по контракту controller'а: terminal state'ы остаются видимыми, чтобы
      // пользователь успел увидеть «только что done»).
      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantTaskUpdate(WsAssistantTaskUpdateEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 6),
          v: 1,
          userId: userId,
          projectId: 'proj-1',
          taskId: 'task-1',
          state: 'done',
          title: 'First task',
          updatedAt: DateTime.utc(2026, 5, 17, 12, 6),
        )),
      ));
      await tester.pumpAndSettle();

      expect(find.text('First task'), findsOneWidget);
      expect(find.text('Second task'), findsOneWidget);
    });

    testWidgets('session_updated → busy=true shows busy indicator',
        (tester) async {
      stubBootstrap();

      await tester.pumpWidget(appUnderTest());
      await tester.pumpAndSettle();
      final l10n = l10nOf(tester);

      // busy=false → индикатора нет.
      expect(find.text(l10n.assistantSessionBusy), findsNothing,
          reason: 'no busy indicator before WS event');

      wsEvents.add(WsClientEvent.server(
        WsServerEvent.assistantSessionUpdated(WsAssistantSessionUpdatedEvent(
          ts: DateTime.utc(2026, 5, 17, 12, 7),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          status: 'active',
          busy: true,
          updatedAt: DateTime.utc(2026, 5, 17, 12, 7),
        )),
      ));
      // pumpAndSettle здесь не работает: CircularProgressIndicator
      // анимируется бесконечно. Достаточно пары pump'ов.
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));

      expect(find.text(l10n.assistantSessionBusy), findsOneWidget,
          reason: 'busy indicator text from l10n.assistantSessionBusy');
    });
  });
}
