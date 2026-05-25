// Sprint 21 — widget-тест для AssistantChatPanel.
//
// E2E happy-path (empty → send → WS reply, tool_call, confirm_request) уже покрыт
// в `integration/assistant_sidebar_e2e_test.dart`. Здесь добиваем виджет-уровень
// для сценариев, которые не зависят от полного сайдбара:
//
//   1. Первый рендер → controller.ensureSession() ходит за сессией.
//   2. Error banner с кнопкой Retry → tap снимает state.error.
//   3. session.busy=true → busy-индикатор + дизабленный input.
//   4. hasMore=true (через WS append старой страницы — невозможно; вместо этого
//      покрыто отдельным тестом ChatController). Здесь — empty hint, когда
//      история пустая и нет pendingConfirm.
//   5. tap "+" → controller.startNewSession() → repo.createSession.
//
// Моки переиспользуем из контроллер-теста — общий тип, общий codegen.

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
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_chat_panel.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mockito/mockito.dart';

import '../controllers/assistant_chat_controller_test.mocks.dart';

void main() {
  const userId = 'u-1';
  const sessionId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
  const newSessionId = 'cccccccc-cccc-cccc-cccc-cccccccccccc';

  AssistantSessionModel session({String? id, bool busy = false}) =>
      AssistantSessionModel(
        id: id ?? sessionId,
        userId: userId,
        status: 'active',
        busy: busy,
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 2),
      );

  late MockAssistantRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;

  void stubHistory({List<AssistantMessageModel> history = const []}) {
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

  Widget app() => ProviderScope(
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
          theme: ThemeData(
            splashFactory: NoSplash.splashFactory,
          ),
          home: const Scaffold(
            body: SizedBox(width: 400, height: 720, child: AssistantChatPanel()),
          ),
        ),
      );

  group('AssistantChatPanel — bootstrap', () {
    testWidgets(
        'first frame: ensureSession picks existing session and loads history',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => AssistantSessionListResponse(sessions: [session()]));
      stubHistory();

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      // listSessions зовут ДВА разных потребителя: chatController.ensureSession
      // и assistantSessionsListProvider (picker). Проверяем лишь то, что
      // chat-pipeline дошёл до getMessages с корректным sessionId — он
      // эксклюзивен для controller'а.
      verify(mockRepo.getMessages(
        sessionId,
        limit: anyNamed('limit'),
        beforeCreatedAt: anyNamed('beforeCreatedAt'),
        beforeId: anyNamed('beforeId'),
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    testWidgets('no existing sessions → repo.createSession called',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => const AssistantSessionListResponse(sessions: []));
      when(mockRepo.createSession(cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => session());
      stubHistory();

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      verify(mockRepo.createSession(cancelToken: anyNamed('cancelToken')))
          .called(1);
    });

    testWidgets(
        'empty history + no pendingConfirm → renders assistantEmptyChat hint',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => AssistantSessionListResponse(sessions: [session()]));
      stubHistory();

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      expect(
        find.text('Ask the assistant about your projects, tasks, or settings.'),
        findsOneWidget,
      );
    });
  });

  group('AssistantChatPanel — busy state', () {
    testWidgets(
        'session.busy=true → busy hint + send button disabled (isSending)',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async =>
          AssistantSessionListResponse(sessions: [session(busy: true)]));
      stubHistory();

      await tester.pumpWidget(app());
      // Бесконечно крутящийся CircularProgressIndicator (busy-индикатор)
      // ломает pumpAndSettle — поэтому ждём вручную несколькими pump'ами,
      // достаточными для прохода Future-chain в ensureSession.
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));
      await tester.pump();

      // Busy hint текст.
      expect(find.text('Assistant is working…'), findsOneWidget);
      // Send-кнопка ChatInput при isSending=true disabled (onPressed null).
      final sendBtn = find.byKey(const ValueKey('chat_send_button'));
      expect(sendBtn, findsOneWidget);
      final btnWidget = tester.widget<IconButton>(sendBtn);
      expect(btnWidget.onPressed, isNull);

      // Кнопка "+" (новая сессия) тоже не должна быть нажата во время
      // creatingSession — но тут busy от session, кнопка остаётся активной.
      expect(find.byIcon(Icons.add_comment_outlined), findsOneWidget);
    });
  });

  group('AssistantChatPanel — error banner', () {
    testWidgets(
        'ensureSession failure → banner visible, tap Retry hides it',
        (tester) async {
      // Стартовый ensureSession падает на listSessions → controller пишет
      // ошибку в state.error через swallow-path (`unawaited(... catchError)`),
      // не пробрасывая её в UI-handler (там был бы unhandled rethrow).
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(AssistantApiException('boom', statusCode: 500));

      await tester.pumpWidget(app());
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));
      await tester.pump();

      expect(find.text('Something went wrong. Please try again.'),
          findsOneWidget);
      expect(find.text('Retry'), findsOneWidget);

      await tester.tap(find.text('Retry'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));

      expect(find.text('Something went wrong. Please try again.'),
          findsNothing);
    });
  });

  group('AssistantChatPanel — new session', () {
    testWidgets('tap "+" → repo.createSession called', (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => AssistantSessionListResponse(sessions: [session()]));
      stubHistory();
      when(mockRepo.createSession(cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => session(id: newSessionId));

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      await tester.tap(find.byIcon(Icons.add_comment_outlined));
      await tester.pumpAndSettle();

      verify(mockRepo.createSession(cancelToken: anyNamed('cancelToken')))
          .called(1);
    });
  });
}

