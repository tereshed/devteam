// Sprint 21 — widget-тест для AssistantSessionPicker.
//
// PopupMenu тестируем на полное взаимодействие: tap → меню → пункт → selectSession.
// Моки AssistantRepository / WebSocketService переиспользуем из контроллер-теста
// (уже сгенерированы build_runner'ом — не плодим .mocks.dart на тот же тип).

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
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_session_picker.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mockito/mockito.dart';

import '../controllers/assistant_chat_controller_test.mocks.dart';

void main() {
  const userId = 'u-1';
  const sessionAId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
  const sessionBId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';

  AssistantSessionModel session(String id, {String? title}) =>
      AssistantSessionModel(
        id: id,
        userId: userId,
        title: title,
        status: 'active',
        busy: false,
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 2),
      );

  late MockAssistantRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;

  setUp(() {
    mockRepo = MockAssistantRepository();
    mockWs = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(mockWs.events).thenAnswer((_) => wsEvents.stream);
    when(mockRepo.getMessages(
      any,
      limit: anyNamed('limit'),
      beforeCreatedAt: anyNamed('beforeCreatedAt'),
      beforeId: anyNamed('beforeId'),
      cancelToken: anyNamed('cancelToken'),
    )).thenAnswer((_) async => const AssistantMessageListResponse(
          messages: [],
          limit: 30,
          hasMore: false,
        ));
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
          // Picker занимает ширину родителя — даём ему конечный размер.
          home: const Scaffold(
            body: SizedBox(width: 320, child: AssistantSessionPicker()),
          ),
        ),
      );

  group('AssistantSessionPicker', () {
    testWidgets('shows "—" placeholder when no current session', (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => const AssistantSessionListResponse(sessions: []));

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      expect(find.text('—'), findsOneWidget);
      expect(find.byIcon(Icons.arrow_drop_down), findsOneWidget);
    });

    testWidgets(
        'tap → menu lists sessions, tapping item calls repo.getSession',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => AssistantSessionListResponse(
            sessions: [
              session(sessionAId, title: 'Session A'),
              session(sessionBId, title: 'Session B'),
            ],
          ));
      when(mockRepo.getSession(any, cancelToken: anyNamed('cancelToken')))
          .thenAnswer((_) async => session(sessionBId, title: 'Session B'));

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      // Открываем меню.
      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();

      expect(find.text('Session A'), findsOneWidget);
      expect(find.text('Session B'), findsOneWidget);

      // Выбираем B → контроллер должен сходить за полной сессией.
      await tester.tap(find.text('Session B'));
      await tester.pumpAndSettle();

      verify(mockRepo.getSession(sessionBId,
              cancelToken: anyNamed('cancelToken')))
          .called(1);
    });

    testWidgets(
        'session without title renders "Untitled chat" placeholder in menu',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => AssistantSessionListResponse(
            sessions: [session(sessionAId)],
          ));

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();

      expect(find.text('Untitled chat'), findsOneWidget);
    });

    testWidgets('empty sessions list → disabled "Untitled chat" placeholder',
        (tester) async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer(
          (_) async => const AssistantSessionListResponse(sessions: []));

      await tester.pumpWidget(app());
      await tester.pumpAndSettle();

      await tester.tap(find.byType(PopupMenuButton<String>));
      await tester.pumpAndSettle();

      final item = tester.widget<PopupMenuItem<String>>(
        find.byType(PopupMenuItem<String>).first,
      );
      expect(item.enabled, isFalse);
    });
  });
}
