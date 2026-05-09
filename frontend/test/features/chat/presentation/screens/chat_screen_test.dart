import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/data/conversation_exceptions.dart';
import 'package:frontend/features/chat/domain/linked_task_snapshots.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/presentation/controllers/chat_controller.dart';
import 'package:frontend/features/chat/presentation/screens/chat_screen.dart'
    show ChatScreen, ChatScreenScroll, kTasksCrossBranchPushMaxRetries;
import 'package:frontend/features/chat/presentation/widgets/task_status_card.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/domain/models.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mockito/mockito.dart';

import '../../../tasks/helpers/task_fixtures.dart';
import '../../../tasks/presentation/controllers/task_list_controller_test.mocks.dart';
import '../../helpers/chat_fixtures.dart';
import '../../helpers/chat_mocks.mocks.dart';
import '../../helpers/chat_test_router.dart';
import '../../helpers/test_wrappers.dart';

/// Соответствует числу сообщений в первом ответе [getMessages] (сценарий loadOlder).
const kLoadOlderFirstPageMessageCount = 2;

class _StubTaskListForChatShell extends TaskListController {
  _StubTaskListForChatShell(this._seed);
  final TaskListState _seed;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;
}

void main() {
  late MockConversationRepository repo;
  late MockWebSocketService ws;
  late StreamController<WsClientEvent> wsEvents;

  setUp(() {
    repo = MockConversationRepository();
    ws = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(ws.events).thenAnswer((_) => wsEvents.stream);
    when(ws.connect(any)).thenAnswer((_) => wsEvents.stream);
  });

  tearDown(() async {
    await wsEvents.close();
  });

  List<Override> defaultOverrides() => [
        conversationRepositoryProvider.overrideWithValue(repo),
        webSocketServiceProvider.overrideWithValue(ws),
        projectProvider(kTestChatProjectUuid).overrideWith(
          (ref) async => makeTestChatProject(),
        ),
      ];

  Widget buildSubject({
    Locale locale = const Locale('en'),
    TextScaler textScaler = TextScaler.noScaling,
    Widget? home,
  }) =>
      wrapChatMaterialApp(
        locale: locale,
        textScaler: textScaler,
        overrides: defaultOverrides(),
        home: home ??
            const ChatScreen(
              projectId: kTestChatProjectUuid,
              conversationId: kTestChatConversationUuid,
            ),
      );

  ConversationMessageModel assistantMsg(String id, String content) =>
      makeMessage(id: id, role: 'assistant', content: content);

  ConversationMessageModel userMsg(String id, String content) =>
      makeMessage(id: id, role: 'user', content: content);

  testWidgets('smoke: загружает историю и заголовок беседы', (tester) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => makeMessageListResponse(
        messages: [
          assistantMsg('m1', kChatFixtureAssistantHelloWorld),
        ],
      ),
    );

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    expect(find.text(kChatFixtureAssistantHelloWorld), findsOneWidget);
    expect(find.text(kChatFixtureConversationTitle), findsOneWidget);
  });

  testWidgets('транзиентная ошибка отправки → Retry → сообщение в ленте',
      (tester) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeMessageListResponse());

    var sendCalls = 0;
    when(
      repo.sendMessage(
        kTestChatConversationUuid,
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
        message: userMsg('u1', kChatFixtureUserTypedText),
        status: MessageSendStatus.created,
      );
    });

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const ValueKey('chat_input_field')),
      kChatFixtureUserTypedText,
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pumpAndSettle();

    expect(sendCalls, 1);
    final l10n = AppLocalizations.of(
      tester.element(find.byType(ChatScreen)),
    )!;
    expect(find.text(l10n.chatScreenPendingRetry), findsOneWidget);

    await tester.tap(find.text(l10n.chatScreenPendingRetry));
    await tester.pumpAndSettle();

    expect(sendCalls, 2);
    expect(find.text(kChatFixtureUserTypedText), findsWidgets);
  });

  testWidgets('ru-smoke: транзиентная ошибка → chatScreenPendingRetry', (
    tester,
  ) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeMessageListResponse());

    var sendCalls = 0;
    when(
      repo.sendMessage(
        kTestChatConversationUuid,
        any,
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async {
      sendCalls++;
      if (sendCalls == 1) {
        throw ConversationApiException('bad gateway', statusCode: 502);
      }
      return SendMessageResult(
        message: userMsg('u1', 'x'),
        status: MessageSendStatus.created,
      );
    });

    await tester.pumpWidget(buildSubject(locale: const Locale('ru')));
    await tester.pumpAndSettle();

    await tester.enterText(
      find.byKey(const ValueKey('chat_input_field')),
      'text',
    );
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('chat_send_button')));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ChatScreen)),
    )!;
    expect(find.text(l10n.chatScreenPendingRetry), findsOneWidget);
  });

  Widget buildSubjectInShortViewport() => wrapChatMaterialApp(
        overrides: defaultOverrides(),
        home: const SizedBox(
          width: 400,
          height: 260,
          child: ChatScreen(
            projectId: kTestChatProjectUuid,
            conversationId: kTestChatConversationUuid,
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
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [
            assistantMsg('m1', 'first'),
          ],
        ),
      );

      await tester.pumpWidget(buildSubject());
      await tester.pumpAndSettle();

      expect(
        await readListScrollOffset(tester),
        lessThan(ChatScreenScroll.bottomStickPx + 8),
      );

      final ctx = tester.element(find.byType(ChatScreen));
      final container = ProviderScope.containerOf(ctx);
      final notifier = container.read(
        chatControllerProvider(
          projectId: kTestChatProjectUuid,
          conversationId: kTestChatConversationUuid,
        ).notifier,
      );
      notifier.applyIncomingMessage(
        assistantMsg('m2', 'second').copyWith(
          createdAt: DateTime.utc(2026, 1, 5),
        ),
      );
      await tester.pump();
      await tester.pump(ChatScreenScroll.scrollToBottomDuration);
      await tester.pump(const Duration(milliseconds: 16));

      expect(
        await readListScrollOffset(tester),
        lessThan(ChatScreenScroll.bottomStickPx + 8),
      );
    },
  );

  testWidgets(
    'auto-scroll: отмотал вверх — входящее сообщение не притягивает к низу',
    (tester) async {
      final wall = Iterable.generate(200, kChatFixtureWallLine).join('\n');
      when(
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [
            assistantMsg('m1', wall),
            assistantMsg('m2', wall),
          ],
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
        chatControllerProvider(
          projectId: kTestChatProjectUuid,
          conversationId: kTestChatConversationUuid,
        ).notifier,
      );
      notifier.applyIncomingMessage(
        assistantMsg('m3', 'new while reading').copyWith(
          createdAt: DateTime.utc(2026, 1, 6),
        ),
      );
      await tester.pump();
      await tester.pump(ChatScreenScroll.scrollToBottomDuration);
      await tester.pump(const Duration(milliseconds: 16));

      final offsetAfter = await readListScrollOffset(tester);
      expect(offsetAfter, greaterThan(ChatScreenScroll.bottomStickPx));
      expect(
        (offsetAfter - offsetBefore).abs(),
        lessThan(offsetBefore * 0.85),
        reason: 'не должно притягивать к низу как при animateTo(0)',
      );
    },
  );

  testWidgets(
    'смок: вертикальный drag с области code-блока прокручивает ленту',
    (tester) async {
      when(
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      final wall = List.generate(25, kChatFixtureWallLine).join('\n');
      final content = '$wall\n\n```dart\nconst k = 1;\n```';
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [assistantMsg('m1', content)],
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

  testWidgets('ошибка начальной загрузки: chatErrorTitle + retry', (
    tester,
  ) async {
    final apiErr = ConversationApiException(
      'HTTP 503: upstream timeout',
      statusCode: 503,
    );
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(apiErr);

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ChatScreen)),
    )!;
    expect(find.text(chatErrorTitle(l10n, apiErr)), findsOneWidget);
    expect(find.textContaining('upstream timeout'), findsOneWidget);
    expect(find.text(l10n.retry), findsOneWidget);
  });

  testWidgets('пустая лента: список без падения', (tester) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeMessageListResponse());

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('chat_message_list')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('loadOlder: второй запрос с offset после скролла к верху', (
    tester,
  ) async {
    final wall = Iterable.generate(60, kChatFixtureWallLine).join('\n');
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((invocation) async {
      final offset = invocation.namedArguments[#offset] as int;
      if (offset == 0) {
        return makeMessageListResponse(
          messages: [
            assistantMsg('m1', wall),
            assistantMsg('m2', wall),
          ],
          hasNext: true,
        );
      }
      return makeMessageListResponse(
        messages: [
          assistantMsg('old1', kChatFixtureOlderChunkBody),
        ],
        hasNext: false,
      );
    });

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
    expect(pos.maxScrollExtent, greaterThan(ChatScreenScroll.loadOlderLeadPx));

    pos.jumpTo(pos.maxScrollExtent - 4);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    verify(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: kLoadOlderFirstPageMessageCount,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });

  testWidgets('invalidate: после перезагрузки лента снова доступна', (
    tester,
  ) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => makeMessageListResponse(
        messages: [assistantMsg('m1', kChatFixtureAfterInvalidateReloadBody)],
      ),
    );

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatScreen)),
    );
    container.invalidate(
      chatControllerProvider(
        projectId: kTestChatProjectUuid,
        conversationId: kTestChatConversationUuid,
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('chat_message_list')), findsOneWidget);
    expect(find.text(kChatFixtureAfterInvalidateReloadBody), findsOneWidget);
  });

  testWidgets('deep-link: router → ChatScreen и загрузка беседы', (
    tester,
  ) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => makeMessageListResponse(
        messages: [assistantMsg('m1', kChatFixtureDeepLinkAssistantBody)],
      ),
    );

    final router = buildChatTestRouter(
      initialLocation: chatTestPathConversation(
        kTestChatProjectUuid,
        kTestChatConversationUuid,
      ),
    );

    await tester.pumpWidget(
      wrapChatDashboardRouter(
        router: router,
        overrides: defaultOverrides(),
      ),
    );
    await tester.pumpAndSettle();

    verify(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
    expect(find.byType(ChatScreen), findsOneWidget);
    expect(find.text(kChatFixtureDeepLinkAssistantBody), findsOneWidget);
  });

  testWidgets(
    'TaskStatusCard onOpen: shell tasks + URL; Back → список, вкладка tasks (12.5)',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
      const taskId = '11111111-1111-1111-1111-111111111111';
      final mockTasks = MockTaskRepository();
      when(
        mockTasks.getTask(taskId, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer(
        (_) async => TaskModel(
          id: taskId,
          projectId: kTestChatProjectUuid,
          title: 'From chat',
          description: 'd',
          status: 'pending',
          priority: 'medium',
          createdByType: 'user',
          createdById: kTaskFixtureUserId,
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 2),
        ),
      );
      when(
        mockTasks.listTaskMessages(
          taskId,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );
      when(
        mockTasks.listTasks(
          kTestChatProjectUuid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskListResponse(
          tasks: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );

      when(
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [
            makeMessage(
              id: 'mCard',
              role: 'assistant',
              content: 'has task',
              linkedTaskIds: [taskId],
              metadata: {
                kLinkedTaskSnapshotsMetadataKey: {
                  taskId: {
                    'title': 'Linked title',
                    'status': 'pending',
                  },
                },
              },
            ),
          ],
        ),
      );

      final listSeed = TaskListState(
        filter: TaskListFilter.defaults(),
        items: const [],
        total: 0,
        offset: 0,
        isLoadingInitial: false,
      );

      final router = buildChatTestRouter(
        initialLocation: chatTestPathConversation(
          kTestChatProjectUuid,
          kTestChatConversationUuid,
        ),
      );

      await tester.pumpWidget(
        wrapChatDashboardRouter(
          router: router,
          overrides: [
            ...defaultOverrides(),
            taskRepositoryProvider.overrideWithValue(mockTasks),
            taskListControllerProvider.overrideWith(
              () => _StubTaskListForChatShell(listSeed),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byType(TaskStatusCard));
      await tester.pumpAndSettle();

      expect(
        router.state.uri.path,
        '/projects/$kTestChatProjectUuid/tasks/$taskId',
      );
      var rail = tester.widget<NavigationRail>(find.byType(NavigationRail));
      expect(rail.selectedIndex, 1);

      await tester.tap(find.byType(BackButton));
      await tester.pumpAndSettle();

      expect(
        router.state.uri.path,
        '/projects/$kTestChatProjectUuid/tasks',
      );
      rail = tester.widget<NavigationRail>(find.byType(NavigationRail));
      expect(rail.selectedIndex, 1);
    },
  );

  testWidgets(
    'TaskStatusCard без StatefulShell: shell==null → push на полный путь',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
      const taskId = '11111111-1111-1111-1111-111111111111';
      final mockTasks = MockTaskRepository();
      when(
        mockTasks.getTask(taskId, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer(
        (_) async => TaskModel(
          id: taskId,
          projectId: kTestChatProjectUuid,
          title: 'No shell',
          description: 'd',
          status: 'pending',
          priority: 'medium',
          createdByType: 'user',
          createdById: kTaskFixtureUserId,
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 2),
        ),
      );
      when(
        mockTasks.listTaskMessages(
          taskId,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );
      when(
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [
            makeMessage(
              id: 'mShell',
              role: 'assistant',
              content: 'x',
              linkedTaskIds: [taskId],
              metadata: {
                kLinkedTaskSnapshotsMetadataKey: {
                  taskId: {
                    'title': 'T',
                    'status': 'pending',
                  },
                },
              },
            ),
          ],
        ),
      );

      final router = GoRouter(
        initialLocation: chatTestPathConversation(
          kTestChatProjectUuid,
          kTestChatConversationUuid,
        ),
        routes: [
          GoRoute(
            path: '/projects/:projectId/chat/:conversationId',
            pageBuilder: (context, state) => NoTransitionPage<void>(
              child: ChatScreen(
                projectId: state.pathParameters['projectId']!,
                conversationId: state.pathParameters['conversationId']!,
              ),
            ),
          ),
          GoRoute(
            path: '/projects/:projectId/tasks/:taskId',
            pageBuilder: (context, state) => const NoTransitionPage<void>(
              child: Scaffold(
                body: Text('__NO_SHELL_TASK_DETAIL__'),
              ),
            ),
          ),
        ],
      );

      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            ...defaultOverrides(),
            taskRepositoryProvider.overrideWithValue(mockTasks),
          ],
          child: MaterialApp.router(
            locale: const Locale('en'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byType(TaskStatusCard));
      await tester.pumpAndSettle();

      expect(
        router.state.uri.path,
        '/projects/$kTestChatProjectUuid/tasks/$taskId',
      );
      expect(find.text('__NO_SHELL_TASK_DETAIL__'), findsOneWidget);
    },
  );

  testWidgets(
    'TaskStatusCard onOpen: detail URL после первых кадров без полного settle (race guard)',
    (tester) async {
      useViewSize(tester, const Size(900, 800));
      const taskId = '11111111-1111-1111-1111-111111111111';
      final mockTasks = MockTaskRepository();
      when(
        mockTasks.getTask(taskId, cancelToken: anyNamed('cancelToken')),
      ).thenAnswer(
        (_) async => TaskModel(
          id: taskId,
          projectId: kTestChatProjectUuid,
          title: 'Race',
          description: 'd',
          status: 'pending',
          priority: 'medium',
          createdByType: 'user',
          createdById: kTaskFixtureUserId,
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 2),
        ),
      );
      when(
        mockTasks.listTaskMessages(
          taskId,
          messageType: anyNamed('messageType'),
          senderType: anyNamed('senderType'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskMessageListResponse(
          messages: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );
      when(
        mockTasks.listTasks(
          kTestChatProjectUuid,
          filter: anyNamed('filter'),
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => const TaskListResponse(
          tasks: [],
          total: 0,
          limit: 50,
          offset: 0,
        ),
      );

      when(
        repo.getConversation(
          kTestChatConversationUuid,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => makeConversation());
      when(
        repo.getMessages(
          kTestChatConversationUuid,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeMessageListResponse(
          messages: [
            makeMessage(
              id: 'mRace',
              role: 'assistant',
              content: 'race',
              linkedTaskIds: [taskId],
              metadata: {
                kLinkedTaskSnapshotsMetadataKey: {
                  taskId: {
                    'title': 'R',
                    'status': 'pending',
                  },
                },
              },
            ),
          ],
        ),
      );

      final listSeed = TaskListState(
        filter: TaskListFilter.defaults(),
        items: const [],
        total: 0,
        offset: 0,
        isLoadingInitial: false,
      );

      final router = buildChatTestRouter(
        initialLocation: chatTestPathConversation(
          kTestChatProjectUuid,
          kTestChatConversationUuid,
        ),
      );

      await tester.pumpWidget(
        wrapChatDashboardRouter(
          router: router,
          overrides: [
            ...defaultOverrides(),
            taskRepositoryProvider.overrideWithValue(mockTasks),
            taskListControllerProvider.overrideWith(
              () => _StubTaskListForChatShell(listSeed),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();

      await tester.tap(find.byType(TaskStatusCard));
      // По одному pump на каждую попытку endOfFrame в [_pushTaskDetailWhenTasksNavigatorReady].
      for (var i = 0; i < kTasksCrossBranchPushMaxRetries; i++) {
        await tester.pump();
      }

      expect(
        router.state.uri.path,
        '/projects/$kTestChatProjectUuid/tasks/$taskId',
      );
    },
  );

  testWidgets('маршрут без conversationId: подсказка выбора беседы', (
    tester,
  ) async {
    final router = buildChatTestRouter(
      initialLocation: chatTestPathChatPlaceholder(kTestChatProjectUuid),
    );

    await tester.pumpWidget(
      wrapChatDashboardRouter(
        router: router,
        overrides: defaultOverrides(),
      ),
    );
    await tester.pumpAndSettle();

    final placeholder = find.byKey(
      const ValueKey('chat-placeholder-$kTestChatProjectUuid'),
    );
    expect(placeholder, findsOneWidget);
    final l10n = AppLocalizations.of(tester.element(placeholder))!;
    expect(find.text(l10n.chatScreenSelectConversationHint), findsOneWidget);
  });

  testWidgets('адаптив: TextScaler 1.5 — send hitTestable, без overflow', (
    tester,
  ) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    final long = List.filled(12, 'Word ').join() * 8;
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => makeMessageListResponse(
        messages: [assistantMsg('m1', long)],
      ),
    );

    await tester.pumpWidget(
      buildSubject(textScaler: const TextScaler.linear(1.5)),
    );
    await tester.pumpAndSettle();

    expect(tester.takeException(), isNull);
    expect(
      find.byKey(const ValueKey('chat_send_button')).hitTestable(),
      findsOneWidget,
    );
  });

  testWidgets('a11y: семантика пузыря user и assistant', (tester) async {
    when(
      repo.getConversation(
        kTestChatConversationUuid,
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) async => makeConversation());
    when(
      repo.getMessages(
        kTestChatConversationUuid,
        limit: anyNamed('limit'),
        offset: anyNamed('offset'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => makeMessageListResponse(
        messages: [
          userMsg('u1', kChatFixtureSemanticHelloUser),
          assistantMsg('a1', kChatFixtureSemanticHelloAssistant),
        ],
      ),
    );

    await tester.pumpWidget(buildSubject());
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ChatScreen)),
    )!;
    expect(
      find.bySemanticsLabel(
        l10n.chatScreenMessageSemanticUser(kChatFixtureSemanticHelloUser),
      ),
      findsOneWidget,
    );
    expect(
      find.bySemanticsLabel(
        l10n.chatScreenMessageSemanticAssistant(
          kChatFixtureSemanticHelloAssistant,
        ),
      ),
      findsOneWidget,
    );
  });
}
