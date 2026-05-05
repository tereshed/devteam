import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/chat/presentation/controllers/chat_controller.dart';
import 'package:frontend/features/chat/presentation/screens/chat_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mockito/mockito.dart';

import '../controllers/chat_controller_test.mocks.dart';

void main() {
  const pid = '550e8400-e29b-41d4-a716-446655440000';
  const cid = '6ba7b810-9dad-11d1-80b4-00c04fd430c8';

  late MockConversationRepository repo;

  ConversationModel conv() => ConversationModel(
        id: cid,
        projectId: pid,
        title: 'Chat title',
        status: 'active',
        createdAt: DateTime.utc(2026, 1, 1),
        updatedAt: DateTime.utc(2026, 1, 2),
      );

  ConversationMessageModel assistantMsg(String id, String content) =>
      ConversationMessageModel(
        id: id,
        conversationId: cid,
        role: 'assistant',
        content: content,
        createdAt: DateTime.utc(2026, 1, 3),
      );

  ConversationMessageModel userMsg(String id, String content) =>
      ConversationMessageModel(
        id: id,
        conversationId: cid,
        role: 'user',
        content: content,
        createdAt: DateTime.utc(2026, 1, 4),
      );

  setUp(() {
    repo = MockConversationRepository();
  });

  Widget buildSubject() => ProviderScope(
        overrides: [
          conversationRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(
          localizationsDelegates: [
            AppLocalizations.delegate,
            GlobalMaterialLocalizations.delegate,
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          supportedLocales: [Locale('en')],
          home: ChatScreen(projectId: pid, conversationId: cid),
        ),
      );

  testWidgets('smoke: загружает историю и заголовок беседы', (tester) async {
    when(
      repo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
    ).thenAnswer((_) async => conv());
    when(
      repo.getMessages(
        cid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => MessageListResponse(
        messages: [assistantMsg('m1', 'Hello world')],
        hasNext: false,
      ),
    );

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    expect(find.text('Hello world'), findsOneWidget);
    expect(find.text('Chat title'), findsOneWidget);
  });

  testWidgets('транзиентная ошибка отправки → Retry → сообщение в ленте',
      (tester) async {
    when(
      repo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
    ).thenAnswer((_) async => conv());
    when(
      repo.getMessages(
        cid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => const MessageListResponse());

    var sendCalls = 0;
    when(
      repo.sendMessage(
        cid,
        any,
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async {
      sendCalls++;
      if (sendCalls == 1) {
        throw ConversationApiException(
          'bad gateway',
          statusCode: 502,
        );
      }
      return SendMessageResult(
        message: userMsg('u1', 'typed text'),
        status: MessageSendStatus.created,
      );
    });

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const ValueKey('chat_input_field')),
      'typed text',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pumpAndSettle();

    expect(sendCalls, 1);
    expect(find.text('Retry send'), findsOneWidget);

    await tester.tap(find.text('Retry send'));
    await tester.pumpAndSettle();

    expect(sendCalls, 2);
    expect(find.text('typed text'), findsWidgets);
  });

  Widget buildSubjectInShortViewport() => ProviderScope(
        overrides: [
          conversationRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(
          localizationsDelegates: [
            AppLocalizations.delegate,
            GlobalMaterialLocalizations.delegate,
            GlobalWidgetsLocalizations.delegate,
            GlobalCupertinoLocalizations.delegate,
          ],
          supportedLocales: [Locale('en')],
          home: SizedBox(
            width: 400,
            height: 260,
            child: ChatScreen(projectId: pid, conversationId: cid),
          ),
        ),
      );

  Future<double> readListScrollOffset(WidgetTester tester) async {
    final inner = find.descendant(
      of: find.byKey(const ValueKey('chat_message_list')),
      matching: find.byType(Scrollable),
    );
    expect(inner, findsWidgets);
    final widgets = tester.widgetList<Scrollable>(inner);
    for (final s in widgets) {
      final c = s.controller;
      if (c != null &&
          c.hasClients &&
          c.position.maxScrollExtent > 8) {
        return c.offset;
      }
    }
    return widgets.first.controller!.offset;
  }

  testWidgets(
    'auto-scroll: у низа после входящего сообщения offset остаётся у низа',
    (tester) async {
      when(
        repo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        repo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [
            assistantMsg('m1', 'first'),
          ],
          hasNext: false,
        ),
      );

      await tester.pumpWidget(buildSubject());
      await tester.pumpAndSettle();

      expect(await readListScrollOffset(tester), lessThan(ChatScreenScroll.bottomStickPx + 8));

      final ctx = tester.element(find.byType(ChatScreen));
      final container = ProviderScope.containerOf(ctx);
      final notifier = container.read(
        chatControllerProvider(projectId: pid, conversationId: cid).notifier,
      );
      notifier.applyIncomingMessage(
        assistantMsg('m2', 'second').copyWith(
          createdAt: DateTime.utc(2026, 1, 5),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      expect(await readListScrollOffset(tester), lessThan(ChatScreenScroll.bottomStickPx + 8));
    },
  );

  testWidgets(
    'auto-scroll: отмотал вверх — входящее сообщение не притягивает к низу',
    (tester) async {
      final wall = Iterable.generate(200, (i) => 'Line $i ${'x' * 80}').join('\n');
      when(
        repo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      when(
        repo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [
            assistantMsg('m1', wall),
            assistantMsg('m2', wall),
          ],
          hasNext: false,
        ),
      );

      await tester.pumpWidget(buildSubjectInShortViewport());
      await tester.pumpAndSettle();

      final scrollFinder = find
          .descendant(
            of: find.byKey(const ValueKey('chat_message_list')),
            matching: find.byType(Scrollable),
          )
          .first;
      final scrollableState = tester.state<ScrollableState>(scrollFinder);
      final pos = scrollableState.position;
      expect(pos.maxScrollExtent, greaterThan(48));
      pos.jumpTo(pos.maxScrollExtent * 0.55);
      await tester.pump();

      final offsetBefore = pos.pixels;
      expect(offsetBefore, greaterThan(ChatScreenScroll.bottomStickPx));

      final ctx = tester.element(find.byType(ChatScreen));
      final container = ProviderScope.containerOf(ctx);
      final notifier = container.read(
        chatControllerProvider(projectId: pid, conversationId: cid).notifier,
      );
      notifier.applyIncomingMessage(
        assistantMsg('m3', 'new while reading').copyWith(
          createdAt: DateTime.utc(2026, 1, 6),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 300));

      final offsetAfter = await readListScrollOffset(tester);
      expect(offsetAfter, greaterThan(ChatScreenScroll.bottomStickPx));
      expect(
        (offsetAfter - offsetBefore).abs(),
        lessThan(offsetBefore * 0.85),
        reason: 'не должно притягивать к низу как при animateTo(0)',
      );
    },
  );

  /// Страховка жестов: горизонтальный скролл кода + вертикальная лента ([ChatScreen], 11.5);
  /// блок кода — [ChatMessage] / `chat_message_builders.dart` (11.6).
  testWidgets(
    'смок: вертикальный drag с области code-блока прокручивает ленту',
    (tester) async {
      when(
        repo.getConversation(cid, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer((_) async => conv());
      final wall = List.generate(25, (i) => 'Line $i ${'x' * 72}').join('\n');
      final content = '$wall\n\n```dart\nconst k = 1;\n```';
      when(
        repo.getMessages(
          cid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => MessageListResponse(
          messages: [assistantMsg('m1', content)],
          hasNext: false,
        ),
      );

      await tester.pumpWidget(buildSubjectInShortViewport());
      await tester.pumpAndSettle();

      final codeFinder =
          find.byKey(const ValueKey<String>('chat_message_code_hscroll'));
      await tester.ensureVisible(codeFinder);
      await tester.pump();

      final scrollFinder = find
          .descendant(
            of: find.byKey(const ValueKey('chat_message_list')),
            matching: find.byType(Scrollable),
          )
          .first;
      final scrollState = tester.state<ScrollableState>(scrollFinder);
      final pos = scrollState.position;
      expect(pos.maxScrollExtent, greaterThan(48));

      final before = pos.pixels;
      await tester.drag(
        codeFinder,
        const Offset(0, -200),
      );
      await tester.pumpAndSettle();

      final after = pos.pixels;
      expect(
        (after - before).abs(),
        greaterThan(4.0),
        reason: 'вертикальный drag с code-блока (11.5 + 11.6) не должен блокировать scroll ленты',
      );
    },
  );
}
