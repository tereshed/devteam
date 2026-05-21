// @dart=2.19
@TestOn('vm')
@Tags(['unit'])
library;

import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/assistant/data/assistant_exceptions.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/assistant/data/assistant_repository.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/domain/assistant_session_model.dart';
import 'package:frontend/features/assistant/presentation/controllers/assistant_chat_controller.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:fake_async/fake_async.dart';

import 'assistant_chat_controller_test.mocks.dart';

@GenerateNiceMocks([
  MockSpec<AssistantRepository>(),
  MockSpec<WebSocketService>(),
])
void main() {
  const sessionId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
  const userId = '11111111-1111-1111-1111-111111111111';
  const messageId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';
  const otherSessionId = 'cccccccc-cccc-cccc-cccc-cccccccccccc';

  late MockAssistantRepository mockRepo;
  late MockWebSocketService mockWs;
  late StreamController<WsClientEvent> wsEvents;
  late ProviderContainer container;

  AssistantSessionModel session({String? id, bool busy = false}) {
    return AssistantSessionModel(
      id: id ?? sessionId,
      userId: userId,
      status: 'active',
      busy: busy,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 2),
    );
  }

  AssistantMessageModel msg(
    String id,
    String content,
    DateTime createdAt, {
    String role = 'user',
    String? sid,
  }) {
    return AssistantMessageModel(
      id: id,
      sessionId: sid ?? sessionId,
      role: role,
      content: content,
      createdAt: createdAt,
    );
  }

  AssistantChatController ctrl() =>
      container.read(assistantChatControllerProvider.notifier);

  AssistantChatState st() =>
      container.read(assistantChatControllerProvider);

  setUp(() {
    mockRepo = MockAssistantRepository();
    mockWs = MockWebSocketService();
    wsEvents = StreamController<WsClientEvent>.broadcast();
    when(mockWs.events).thenAnswer((_) => wsEvents.stream);
    container = ProviderContainer(
      overrides: [
        assistantRepositoryProvider.overrideWithValue(mockRepo),
        webSocketServiceProvider.overrideWithValue(mockWs),
      ],
    );
    addTearDown(() async {
      await wsEvents.close();
      container.dispose();
    });
  });

  group('AssistantChatController.ensureSession', () {
    test('reuses most-recent active session when listSessions returns one',
        () async {
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
      )).thenAnswer((_) async => const AssistantMessageListResponse(
            messages: [],
            limit: 30,
            hasMore: false,
          ));

      final id = await ctrl().ensureSession();

      expect(id, sessionId);
      expect(st().currentSessionId, sessionId);
      expect(st().session?.id, sessionId);
      verifyNever(mockRepo.createSession(
        cancelToken: anyNamed('cancelToken'),
      ));
    });

    test('creates a new session when listSessions is empty', () async {
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => const AssistantSessionListResponse());
      when(mockRepo.createSession(
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => session());
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

      final id = await ctrl().ensureSession();

      expect(id, sessionId);
      verify(mockRepo.createSession(
        cancelToken: anyNamed('cancelToken'),
      )).called(1);
    });

    test('returns cached sessionId on second call (no extra REST)', () async {
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
      )).thenAnswer((_) async => const AssistantMessageListResponse(
            messages: [],
            limit: 30,
            hasMore: false,
          ));

      await ctrl().ensureSession();
      clearInteractions(mockRepo);

      final id = await ctrl().ensureSession();
      expect(id, sessionId);
      verifyZeroInteractions(mockRepo);
    });

    test('REST failure leaves error in state and rethrows', () async {
      final ex = AssistantApiException('boom', statusCode: 500);
      when(mockRepo.listSessions(
        limit: anyNamed('limit'),
        includeArchived: anyNamed('includeArchived'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(ex);

      await expectLater(
        ctrl().ensureSession(),
        throwsA(equals(ex)),
      );
      expect(st().error, equals(ex));
      expect(st().creatingSession, isFalse);
    });
  });

  group('AssistantChatController.sendMessage', () {
    test('appends user message via single upsert (no duplicate branching)',
        () async {
      _wireSessionBootstrap(mockRepo, session());

      final userMsg = msg(messageId, 'hello', DateTime.utc(2026, 1, 3));
      when(mockRepo.sendMessage(
        any,
        content: anyNamed('content'),
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => SendAssistantMessageResponse(
            message: userMsg,
            duplicate: false,
          ));

      await ctrl().ensureSession();
      await ctrl().sendMessage('hello');

      expect(st().messages.length, 1);
      expect(st().messages.first.id, messageId);
      expect(st().sending, isFalse);
    });

    test('idempotent duplicate does not double-append', () async {
      _wireSessionBootstrap(mockRepo, session());

      final userMsg = msg(messageId, 'hello', DateTime.utc(2026, 1, 3));
      when(mockRepo.sendMessage(
        any,
        content: anyNamed('content'),
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async => SendAssistantMessageResponse(
            message: userMsg,
            duplicate: true,
          ));

      await ctrl().ensureSession();
      await ctrl().sendMessage('hello');
      await ctrl().sendMessage('hello');

      // Те же id → upsert не плодит дубль.
      expect(st().messages.map((m) => m.id).toList(), [messageId]);
    });

    test('empty input is no-op', () async {
      await ctrl().sendMessage('   ');
      verifyZeroInteractions(mockRepo);
    });

    test('does nothing when session is busy', () async {
      _wireSessionBootstrap(mockRepo, session(busy: true));
      await ctrl().ensureSession();
      clearInteractions(mockRepo);

      await ctrl().sendMessage('hello');

      verifyNever(mockRepo.sendMessage(
        any,
        content: anyNamed('content'),
        clientMessageId: anyNamed('clientMessageId'),
        cancelToken: anyNamed('cancelToken'),
      ));
    });
  });

  group('AssistantChatController WS event merging', () {
    test('filters events for foreign sessionId', () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantMessage(
        WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 1, 5),
          v: 1,
          userId: userId,
          sessionId: otherSessionId,
          messageId: 'zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz',
          role: 'assistant',
          content: 'leak',
          createdAt: DateTime.utc(2026, 1, 5),
        ),
      )));
      await _drain();

      expect(st().messages, isEmpty);
    });

    test('assistant.message inserts into ASC list and dedups by id', () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      void emit(String id, DateTime at, {String? content}) {
        wsEvents.add(WsClientEvent.server(WsServerEvent.assistantMessage(
          WsAssistantMessageEvent(
            ts: at,
            v: 1,
            userId: userId,
            sessionId: sessionId,
            messageId: id,
            role: 'assistant',
            content: content,
            createdAt: at,
          ),
        )));
      }

      emit('mid-2', DateTime.utc(2026, 1, 3, 12), content: 'b');
      emit('mid-1', DateTime.utc(2026, 1, 3, 11), content: 'a');
      // Дубликат должен переписать, не добавить.
      emit('mid-1', DateTime.utc(2026, 1, 3, 11), content: 'a-updated');
      await _drain();

      expect(st().messages.map((m) => m.id).toList(), ['mid-1', 'mid-2']);
      expect(st().messages.first.content, 'a-updated');
    });

    test('tool_call enriches existing assistant row with arguments', () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      // 1) assistant.message с tool_call_id (но без arguments).
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantMessage(
        WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          messageId: 'asst-1',
          role: 'assistant',
          toolCallId: 'tc-1',
          toolName: 'project_list',
          createdAt: DateTime.utc(2026, 1, 3, 12),
        ),
      )));
      // 2) assistant.tool_call с arguments.
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantToolCall(
        WsAssistantToolCallEvent(
          ts: DateTime.utc(2026, 1, 3, 12, 0, 1),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_list',
          arguments: const {'limit': 10},
        ),
      )));
      await _drain();

      final row = st().messages.firstWhere((m) => m.id == 'asst-1');
      expect(row.toolArguments, equals(<String, dynamic>{'limit': 10}));
    });

    test('tool_result inserts a tool-row that pairs with assistant row',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantMessage(
        WsAssistantMessageEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          messageId: 'asst-1',
          role: 'assistant',
          toolCallId: 'tc-1',
          toolName: 'project_list',
          createdAt: DateTime.utc(2026, 1, 3, 12),
        ),
      )));
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantToolResult(
        WsAssistantToolResultEvent(
          ts: DateTime.utc(2026, 1, 3, 12, 0, 2),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_list',
          status: 'ok',
          result: const {'items': []},
        ),
      )));
      await _drain();

      final groups = groupAssistantMessages(st().messages);
      expect(groups.length, 1);
      final g = groups.first;
      expect(g.isToolCall, isTrue);
      expect(g.toolResult, isNotNull);
      expect(g.toolResult!.toolResult?['status'], 'ok');
    });

    test('confirm_request populates pendingConfirm; clearError unaffected',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantConfirmRequest(
        WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_delete',
          arguments: const {'id': 'p1'},
          summary: 'Delete project p1?',
        ),
      )));
      await _drain();

      expect(st().pendingConfirm, isNotNull);
      expect(st().pendingConfirm!.toolCallId, 'tc-1');
    });

    test('navigate event populates pendingNavigate; consume clears it',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantNavigate(
        WsAssistantNavigateEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          route: '/projects/p1',
        ),
      )));
      await _drain();

      expect(st().pendingNavigate?.route, '/projects/p1');
      ctrl().consumeNavigate();
      expect(st().pendingNavigate, isNull);
    });

    test('session_updated keeps existing session fields when null in event',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();
      expect(st().session?.busy, isFalse);

      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantSessionUpdated(
        WsAssistantSessionUpdatedEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          status: 'active',
          busy: true,
          updatedAt: DateTime.utc(2026, 1, 3, 12),
        ),
      )));
      await _drain();

      expect(st().session?.busy, isTrue);
      expect(st().isBusy, isTrue);
    });
  });

  group('AssistantChatController.confirmToolCall', () {
    test('clears pendingConfirm on success', () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();

      // Поставили pendingConfirm через WS.
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantConfirmRequest(
        WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_delete',
        ),
      )));
      await _drain();
      expect(st().pendingConfirm, isNotNull);

      when(mockRepo.confirmToolCall(
        any,
        toolCallId: anyNamed('toolCallId'),
        approved: anyNamed('approved'),
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenAnswer((_) async =>
          const ConfirmToolCallResponse(accepted: true));

      await ctrl().confirmToolCall(toolCallId: 'tc-1', approved: true);

      expect(st().pendingConfirm, isNull);
    });

    test('treats already_confirmed as success (clears pendingConfirm)',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantConfirmRequest(
        WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_delete',
        ),
      )));
      await _drain();

      when(mockRepo.confirmToolCall(
        any,
        toolCallId: anyNamed('toolCallId'),
        approved: anyNamed('approved'),
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(AssistantAlreadyConfirmedException('dup'));

      await ctrl().confirmToolCall(toolCallId: 'tc-1', approved: true);

      expect(st().pendingConfirm, isNull);
      expect(st().error, isNull);
    });

    test('keeps pendingConfirm on generic error and stores in state.error',
        () async {
      _wireSessionBootstrap(mockRepo, session());
      await ctrl().ensureSession();
      wsEvents.add(WsClientEvent.server(WsServerEvent.assistantConfirmRequest(
        WsAssistantConfirmRequestEvent(
          ts: DateTime.utc(2026, 1, 3, 12),
          v: 1,
          userId: userId,
          sessionId: sessionId,
          toolCallId: 'tc-1',
          toolName: 'project_delete',
        ),
      )));
      await _drain();

      final ex = AssistantApiException('500', statusCode: 500);
      when(mockRepo.confirmToolCall(
        any,
        toolCallId: anyNamed('toolCallId'),
        approved: anyNamed('approved'),
        clientRequestId: anyNamed('clientRequestId'),
        cancelToken: anyNamed('cancelToken'),
      )).thenThrow(ex);

      await ctrl().confirmToolCall(toolCallId: 'tc-1', approved: true);
      expect(st().pendingConfirm, isNotNull);
      expect(st().error, equals(ex));
    });
  });

  group('AssistantChatController.polling', () {
    test('starts polling when session is busy, merges messages, and stops when no longer busy', () {
      fakeAsync((async) {
        final initialSession = session(busy: false);
        _wireSessionBootstrap(mockRepo, initialSession);

        final userMsg = msg(messageId, 'hello', DateTime.utc(2026, 1, 3));
        when(mockRepo.sendMessage(
          any,
          content: anyNamed('content'),
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        )).thenAnswer((_) async => SendAssistantMessageResponse(
              message: userMsg,
              duplicate: false,
            ));

        var pollCount = 0;
        final responseMsg = msg('resp-id', 'hello user', DateTime.utc(2026, 1, 4), role: 'assistant');

        when(mockRepo.getSession(sessionId, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async {
          pollCount++;
          // busy on first poll, idle on second poll
          return session(busy: pollCount < 2);
        });

        when(mockRepo.getMessages(
          sessionId,
          limit: anyNamed('limit'),
          beforeCreatedAt: anyNamed('beforeCreatedAt'),
          beforeId: anyNamed('beforeId'),
          cancelToken: anyNamed('cancelToken'),
        )).thenAnswer((_) async => AssistantMessageListResponse(
              messages: pollCount == 1 ? [responseMsg] : [],
              limit: 30,
              hasMore: false,
            ));

        // Start session & send message
        ctrl().ensureSession();
        async.flushMicrotasks();

        ctrl().sendMessage('hello');
        async.flushMicrotasks();

        // Initially we only have userMsg
        expect(st().messages.length, 1);
        expect(st().messages.first.id, messageId);

        // Elapse 2 seconds to trigger first poll
        async.elapse(const Duration(seconds: 2));

        // Now we should have responseMsg because of first poll
        expect(st().messages.length, 2);
        expect(st().messages.any((m) => m.id == 'resp-id'), isTrue);
        expect(st().session?.busy, isTrue);

        // Elapse 2 more seconds to trigger second poll (which returns busy: false)
        async.elapse(const Duration(seconds: 2));

        expect(st().session?.busy, isFalse);

        // Elapse another 2 seconds to make sure it stopped polling
        final initialPollCount = pollCount;
        async.elapse(const Duration(seconds: 2));
        expect(pollCount, initialPollCount);
      });
    });

    test('reconstructs synthetic WsAssistantConfirmRequestEvent when tool approval is pending', () {
      fakeAsync((async) {
        final initialSession = session(busy: false);
        _wireSessionBootstrap(mockRepo, initialSession);

        final userMsg = msg(messageId, 'delete project', DateTime.utc(2026, 1, 3));
        when(mockRepo.sendMessage(
          any,
          content: anyNamed('content'),
          clientMessageId: anyNamed('clientMessageId'),
          cancelToken: anyNamed('cancelToken'),
        )).thenAnswer((_) async => SendAssistantMessageResponse(
              message: userMsg,
              duplicate: false,
            ));

        // Tool call message that requires confirmation
        final toolCallMsg = AssistantMessageModel(
          id: 'tc-msg-id',
          sessionId: sessionId,
          role: 'assistant',
          content: 'I need to delete the project.',
          toolCallId: 'tc-1',
          toolName: 'project_delete',
          toolArguments: const {'projectId': 'p1'},
          createdAt: DateTime.utc(2026, 1, 4),
        );

        when(mockRepo.getSession(sessionId, cancelToken: anyNamed('cancelToken')))
            .thenAnswer((_) async => AssistantSessionModel(
                  id: sessionId,
                  userId: userId,
                  status: 'active',
                  busy: true,
                  pendingToolCallId: 'tc-1',
                  createdAt: DateTime.utc(2026, 1, 1),
                  updatedAt: DateTime.utc(2026, 1, 2),
                ));

        when(mockRepo.getMessages(
          sessionId,
          limit: anyNamed('limit'),
          beforeCreatedAt: anyNamed('beforeCreatedAt'),
          beforeId: anyNamed('beforeId'),
          cancelToken: anyNamed('cancelToken'),
        )).thenAnswer((_) async => AssistantMessageListResponse(
              messages: [toolCallMsg],
              limit: 30,
              hasMore: false,
            ));

        ctrl().ensureSession();
        async.flushMicrotasks();

        ctrl().sendMessage('delete project');
        async.flushMicrotasks();

        expect(st().pendingConfirm, isNull);

        // Elapse 2 seconds to trigger poll
        async.elapse(const Duration(seconds: 2));

        expect(st().pendingConfirm, isNotNull);
        expect(st().pendingConfirm!.toolCallId, 'tc-1');
        expect(st().pendingConfirm!.toolName, 'project_delete');
        expect(st().pendingConfirm!.arguments, const {'projectId': 'p1'});
        expect(st().pendingConfirm!.summary, 'I need to delete the project.');
      });
    });
  });
}

void _wireSessionBootstrap(
  MockAssistantRepository repo,
  AssistantSessionModel s,
) {
  when(repo.listSessions(
    limit: anyNamed('limit'),
    includeArchived: anyNamed('includeArchived'),
    cancelToken: anyNamed('cancelToken'),
  )).thenAnswer((_) async => AssistantSessionListResponse(sessions: [s]));
  when(repo.getMessages(
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
}

Future<void> _drain() async {
  // Прокрутить event loop, чтобы StreamController доставил события.
  for (var i = 0; i < 5; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}
